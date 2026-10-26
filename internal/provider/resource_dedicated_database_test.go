package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type dedicatedOnlyProvider struct{ stackshiftProvider }

func (dedicatedOnlyProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewDedicatedDatabaseResource, NewDedicatedDatabaseReplicaResource, NewDatabaseParameterGroupResource,
		NewDedicatedParameterGroupAttachmentResource, NewDedicatedDatabaseSnapshotResource, NewDedicatedDatabaseRestorePointResource,
		NewDedicatedDatabaseRestoreScheduleResource, NewDedicatedDatabasePublicAccessResource, NewDedicatedDatabaseInsightsResource}
}

func (dedicatedOnlyProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewDatabaseInstanceClassesDataSource, NewDatabaseCredentialsDataSource, NewDatabaseParameterDefinitionsDataSource,
		NewDedicatedDatabaseDataSource, NewDedicatedDatabaseBackupsDataSource}
}

const fakeCheckoutURL = "https://pay.example.test/checkout/abc"
const fakeChargeMinor = int64(150000)

type fakeDedicatedAPI struct {
	mu              sync.Mutex
	paymentRequired bool
	walletEmpty     bool
	created         bool
	deleted         bool
	status          string
	class           string
	pendingClass    string
	replicas        []map[string]any
	confirms        []string
	deleteQuery     string
	createBody      map[string]any
	settings        map[string]any
	settingsPatches []map[string]any
	engine          string
	version         string
	name            string
	nextBilling     string
	billingPuts     []map[string]any
	restoreBody     map[string]any
	management      managementState
}

func (f *fakeDedicatedAPI) database() map[string]any {
	dedicated := map[string]any{"instance_class": f.class}
	for key, value := range f.settings {
		dedicated[key] = value
	}
	engine, version, name := f.engine, f.version, f.name
	if engine == "" {
		engine = "postgres"
	}
	if version == "" {
		version = "17"
	}
	if name == "" {
		name = "orders"
	}
	return map[string]any{"id": "db-1", "tier": "dedicated", "dedicated": dedicated, "name": name, "type": engine, "version": version, "status": f.status,
		"host": "db-1.stackshift.internal", "port": 5432, "tls_mode": "require", "database_name": "app",
		"endpoints": []map[string]any{{"protocol": engine, "network": "private", "role": "primary", "host": "db-1.stackshift.internal", "port": 5432, "tls_mode": "require", "primary": true}}}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeOK(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, map[string]any{"success": true, "data": data})
}

func (f *fakeDedicatedAPI) paid(w http.ResponseWriter, body map[string]any, label string) bool {
	confirm, _ := body["confirm_charge_minor"].(float64)
	f.confirms = append(f.confirms, fmt.Sprintf("%s:%v", label, body["confirm_charge_minor"]))
	if int64(confirm) != fakeChargeMinor {
		writeJSON(w, http.StatusConflict, map[string]any{"success": false, "error": "price_confirmation_required", "message": "This change costs extra.",
			"quote": map[string]any{"current_monthly_minor": 900000, "new_monthly_minor": 1800000, "charge_minor": fakeChargeMinor, "credit_minor": 0, "currency": "NGN", "billing": "monthly"}})
		return false
	}
	if f.walletEmpty {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"success": false, "error": "payment_required", "message": "Top up your wallet to cover this change.",
			"amount_minor": fakeChargeMinor, "currency": "NGN"})
		return false
	}
	return true
}

func (f *fakeDedicatedAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	route := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/v1")
	if strings.HasPrefix(route, "GET /databases/db-1") && (!f.created || f.deleted) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "database not found"})
		return
	}
	switch route {
	case "GET /databases/instance-classes":
		writeOK(w, http.StatusOK, map[string]any{"classes": []map[string]any{
			{"slug": "s-2vcpu-4gb", "vcpu": 2, "memory_mb": 4096, "cpu_policy": "shared", "max_connections": 200, "pooled_max_connections": 2000, "is_active": true,
				"revision": map[string]any{"currency": "NGN", "monthly_price_minor": 900000, "storage_gb_month_price_minor": 20000, "backup_gb_month_price_minor": 5000}},
			{"slug": "d-4vcpu-16gb", "vcpu": 4, "memory_mb": 16384, "cpu_policy": "dedicated", "max_connections": 800, "is_active": true,
				"revision": map[string]any{"currency": "NGN", "monthly_price_minor": 4000000}},
		}})
	case "POST /databases":
		f.created, f.createBody, f.class = true, body, body["instance_class"].(string)
		f.engine, f.version = body["type"].(string), "17"
		if f.engine == "mysql" {
			f.version = "8.4"
		}
		if requested, ok := body["version"].(string); ok {
			f.version = requested
		}
		if body["backup_retention_days"] == nil {
			body["backup_retention_days"] = float64(7)
		}
		window := body["maintenance_window"]
		if window == nil {
			window = map[string]any{"day_of_week": 3, "start_minute": 60}
		}
		f.settings = map[string]any{"storage_gb": body["storage_gb"], "storage_max_gb": body["storage_gb"].(float64) * 2,
			"backup_retention_days": body["backup_retention_days"], "deletion_protection": body["deletion_protection"] == true,
			"maintenance_window": window, "billing": body["billing"], "standby": body["standby"], "on_standby_loss": body["on_standby_loss"]}
		if f.paymentRequired {
			f.status = "pending_payment"
			writeJSON(w, http.StatusPaymentRequired, map[string]any{"success": false, "error": "payment_required", "checkout_url": fakeCheckoutURL,
				"checkout": map[string]any{"status": "payment_required", "payment_url": fakeCheckoutURL, "amount_minor": 900000, "currency": "NGN"},
				"database": f.database(), "master_password": "s3cret-once"})
			return
		}
		f.status = "provisioning"
		writeOK(w, http.StatusAccepted, map[string]any{"database": f.database(), "master_password": "s3cret-once", "checkout": map[string]any{"status": "paid"}})
	case "GET /databases/db-1":
		writeOK(w, http.StatusOK, f.database())
	case "POST /databases/src-1/restore":
		f.created, f.restoreBody, f.class, f.engine, f.version, f.name, f.status = true, body, body["instance_class"].(string), "mysql", "8.4", body["name"].(string), "restoring"
		f.settings = map[string]any{"storage_gb": body["storage_gb"], "storage_max_gb": 80, "backup_retention_days": 14, "deletion_protection": false,
			"maintenance_window": map[string]any{"day_of_week": 1, "start_minute": 30}, "billing": body["billing"], "standby": "off", "on_standby_loss": "degrade"}
		writeOK(w, http.StatusAccepted, map[string]any{"database": f.database(), "master_password": "restored-once", "checkout": map[string]any{"status": "paid"}})
	case "PUT /databases/db-1/billing":
		f.billingPuts = append(f.billingPuts, body)
		f.nextBilling = body["billing"].(string)
		writeOK(w, http.StatusOK, map[string]any{"billing": map[string]any{"mode": f.nextBilling}})
	case "PATCH /databases/db-1":
		f.settingsPatches = append(f.settingsPatches, body)
		for key, value := range body {
			if key == "name" {
				f.name = value.(string)
				continue
			}
			f.settings[key] = value
		}
		writeOK(w, http.StatusOK, f.database())
	case "GET /databases/db-1/billing":
		status := "active"
		if f.status == "pending_payment" {
			status = "pending_payment"
		}
		billing := map[string]any{"mode": "monthly", "status": status, "currency": "NGN"}
		if f.nextBilling != "" {
			billing["next_billing"] = map[string]any{"billing": f.nextBilling, "effective_at": "2026-11-26T00:00:00Z"}
		}
		writeOK(w, http.StatusOK, billing)
	case "GET /databases/db-1/ha":
		members := []map[string]any{}
		if f.status != "pending_payment" {
			members = append(members, map[string]any{"id": "inst-1", "role": "primary", "instance_class": f.class, "status": "running", "site": "hel1"})
		}
		members = append(members, f.replicas...)
		ha := map[string]any{"standby_mode": "off", "on_standby_loss": "degrade", "standby_available": true, "members": members}
		if f.pendingClass != "" {
			ha["size_change"] = map[string]any{"id": "chg-1", "from_class": f.class, "to_class": f.pendingClass, "status": "scheduled"}
		}
		writeOK(w, http.StatusOK, ha)
	case "POST /databases/db-1/class":
		if f.paid(w, body, "class") {
			f.pendingClass = body["instance_class"].(string)
			writeOK(w, http.StatusAccepted, map[string]any{"message": "scheduled"})
		}
	case "POST /databases/db-1/replicas":
		if f.paid(w, body, "replica") {
			f.replicas = append(f.replicas, map[string]any{"id": "rep-1", "role": "replica", "instance_class": "s-2vcpu-4gb", "status": "provisioning", "site": "hel1"})
			writeOK(w, http.StatusAccepted, map[string]any{"replica_id": "rep-1", "instance_class": "s-2vcpu-4gb"})
		}
	case "DELETE /databases/db-1/replicas/rep-1":
		f.replicas = nil
		writeOK(w, http.StatusAccepted, map[string]any{"message": "removing"})
	case "DELETE /databases/db-1":
		f.deleted, f.deleteQuery = true, r.URL.RawQuery
		writeOK(w, http.StatusAccepted, map[string]any{"status": "deleting"})
	default:
		if !f.management.serve(f, w, route, r.URL.Query().Get("family"), body) {
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "no route " + route})
		}
	}
}

func startFakeDedicatedAPI(t *testing.T, fake *fakeDedicatedAPI) {
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("STACKSHIFT_ENDPOINT", server.URL)
	t.Setenv("STACKSHIFT_API_TOKEN", "sspat_test")
}

var dedicatedFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"stackshift": providerserver.NewProtocol6WithError(&dedicatedOnlyProvider{}),
}

func dedicatedDatabaseConfig(class string, extra string) string {
	return fmt.Sprintf(`
resource "stackshift_dedicated_database" "orders" {
  name           = "orders"
  instance_class = %q
  storage_gb     = 20
  maintenance_window = {
    day_of_week  = 2
    start_minute = 120
  }
%s
}
`, class, extra)
}

func TestDedicatedDatabaseInstanceClassesDataSource(t *testing.T) {
	startFakeDedicatedAPI(t, &fakeDedicatedAPI{})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{{
			Config: `data "stackshift_database_instance_classes" "all" {}`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.#", "2"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.0.slug", "s-2vcpu-4gb"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.0.cpu_policy", "shared"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.0.max_connections", "200"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.0.monthly_price_minor", "900000"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.1.cpu_policy", "dedicated"),
				resource.TestCheckResourceAttr("data.stackshift_database_instance_classes.all", "classes.1.currency", "NGN"),
			),
		}},
	})
}

func TestDedicatedDatabaseCreatePaidClassChangeAndDelete(t *testing.T) {
	fake := &fakeDedicatedAPI{}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if fake.deleteQuery != "skip_final_snapshot=true" {
				return fmt.Errorf("delete query = %q", fake.deleteQuery)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: dedicatedDatabaseConfig("s-2vcpu-4gb", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "master_password", "s3cret-once"),
					resource.TestCheckResourceAttr(name, "allocated_storage_gb", "20"),
					resource.TestCheckResourceAttr(name, "status", "provisioning"),
					resource.TestCheckResourceAttr(name, "billing_status", "active"),
					resource.TestCheckResourceAttr(name, "version", "17"),
					resource.TestCheckResourceAttr(name, "standby", "off"),
					resource.TestCheckResourceAttr(name, "host", "db-1.stackshift.internal"),
					resource.TestCheckResourceAttr(name, "endpoints.0.port", "5432"),
					resource.TestCheckNoResourceAttr(name, "checkout_url"),
					func(*terraform.State) error {
						window, _ := fake.createBody["maintenance_window"].(map[string]any)
						if fake.createBody["tier"] != "dedicated" || fake.createBody["billing"] != "monthly" || window["start_minute"] != float64(120) {
							return fmt.Errorf("create body = %v", fake.createBody)
						}
						return nil
					},
				),
			},
			{
				Config:      dedicatedDatabaseConfig("d-4vcpu-16gb", ""),
				ExpectError: regexp.MustCompile(`(?s)costs extra.*NGN 1500\.00.*NGN 18000\.00.*accept_price_changes = true`),
			},
			{
				Config: dedicatedDatabaseConfig("d-4vcpu-16gb", "  accept_price_changes = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "instance_class", "d-4vcpu-16gb"),
					func(*terraform.State) error {
						want := "class:<nil> class:<nil> class:150000"
						if got := strings.Join(fake.confirms, " "); got != want {
							return fmt.Errorf("confirms = %q, want %q", got, want)
						}
						return nil
					},
				),
			},
			{
				Config: strings.Replace(dedicatedDatabaseConfig("d-4vcpu-16gb", "  accept_price_changes = true\n  backup_retention_days = 14\n  deletion_protection = true"),
					"day_of_week  = 2", "day_of_week  = 4", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "backup_retention_days", "14"),
					resource.TestCheckResourceAttr(name, "maintenance_window.day_of_week", "4"),
					func(*terraform.State) error {
						patch := fmt.Sprint(fake.settingsPatches)
						if len(fake.settingsPatches) != 1 || !strings.Contains(patch, "backup_retention_days:14") ||
							!strings.Contains(patch, "deletion_protection:true") || !strings.Contains(patch, "day_of_week:4") {
							return fmt.Errorf("settings patches = %s", patch)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.settings["storage_gb"] = 40
					fake.mu.Unlock()
				},
				Config: dedicatedDatabaseConfig("d-4vcpu-16gb", "  accept_price_changes = true\n  skip_final_snapshot = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue(name, tfjsonpath.New("storage_gb"), knownvalue.Int64Exact(20)),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "skip_final_snapshot", "true"),
					resource.TestCheckResourceAttr(name, "storage_gb", "20"),
					resource.TestCheckResourceAttr(name, "allocated_storage_gb", "40"),
					resource.TestCheckResourceAttr(name, "deletion_protection", "false"),
					resource.TestCheckResourceAttr(name, "maintenance_window.day_of_week", "2"),
				),
			},
			{
				ResourceName: name, ImportState: true, ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"storage_gb", "master_password", "master_username", "accept_price_changes", "apply_changes", "skip_final_snapshot", "checkout_url"},
			},
		},
	})
}

func TestDedicatedDatabaseCreatePaymentRequiredKeepsPendingDatabase(t *testing.T) {
	fake := &fakeDedicatedAPI{paymentRequired: true}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{
				Config: dedicatedDatabaseConfig("s-2vcpu-4gb", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "status", "pending_payment"),
					resource.TestCheckResourceAttr(name, "billing_status", "pending_payment"),
					resource.TestCheckResourceAttr(name, "checkout_url", fakeCheckoutURL),
					resource.TestCheckResourceAttr(name, "master_password", "s3cret-once"),
				),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.status = "provisioning"; fake.mu.Unlock() },
				Config:    dedicatedDatabaseConfig("s-2vcpu-4gb", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "status", "provisioning"),
					resource.TestCheckResourceAttr(name, "billing_status", "active"),
					resource.TestCheckNoResourceAttr(name, "checkout_url"),
					resource.TestCheckResourceAttr(name, "master_password", "s3cret-once"),
				),
			},
		},
	})
}

func TestDedicatedDatabaseCreatePaymentRequiredWarningHasCheckoutURL(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(&fakeDedicatedAPI{paymentRequired: true})
	defer server.Close()
	r := &dedicatedDatabaseResource{client: client.New(server.URL, "sspat_test", nil)}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx)
	plan := tfsdk.Plan{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
	model := dedicatedDatabaseModel{
		ID: types.StringUnknown(), ProjectID: types.StringUnknown(), Name: types.StringValue("orders"), Engine: types.StringUnknown(), Version: types.StringUnknown(),
		InstanceClass: types.StringValue("s-2vcpu-4gb"), StorageGB: types.Int64Value(20), MasterUsername: types.StringUnknown(), DatabaseName: types.StringUnknown(),
		RestoreFrom: types.ObjectNull(map[string]attr.Type{"source_database_id": types.StringType, "target_time": types.StringType, "backup_id": types.StringType, "from_mirror": types.BoolType}), BackupRetentionDays: types.Int64Unknown(), StorageMaxGB: types.Int64Unknown(), AllocatedStorageGB: types.Int64Unknown(), MaintenanceWindow: types.ObjectUnknown(maintenanceWindowType.AttrTypes), DeletionProtection: types.BoolValue(false), Billing: types.StringValue("monthly"),
		Standby: types.StringValue("off"), OnStandbyLoss: types.StringValue("degrade"), ApplyChanges: types.StringValue("window"),
		AcceptPriceChanges: types.BoolValue(false), SkipFinalSnapshot: types.BoolValue(false), MasterPassword: types.StringUnknown(),
		Status: types.StringUnknown(), Host: types.StringUnknown(), Port: types.Int64Unknown(), Endpoints: types.ListUnknown(dedicatedEndpointType),
		BillingStatus: types.StringUnknown(), CheckoutURL: types.StringUnknown(),
	}
	if diags := plan.Set(ctx, &model); diags.HasError() {
		t.Fatal(diags)
	}
	resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("create returned errors: %v", resp.Diagnostics)
	}
	warnings := resp.Diagnostics.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), fakeCheckoutURL) || warnings[0].Severity() != diag.SeverityWarning {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestDedicatedDatabaseReplicaAddAndRemove(t *testing.T) {
	fake := &fakeDedicatedAPI{created: true, status: "running", class: "s-2vcpu-4gb", walletEmpty: true}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database_replica.read"
	config := func(accept bool) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database_replica" "read" {
  database_id          = "db-1"
  accept_price_changes = %t
}
`, accept)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if len(fake.replicas) != 0 {
				return fmt.Errorf("replicas left: %v", fake.replicas)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config(false), ExpectError: regexp.MustCompile(`(?s)read replica costs extra.*NGN 1500\.00`)},
			{Config: config(true), ExpectError: regexp.MustCompile(`(?s)Top up your wallet`)},
			{
				PreConfig: func() { fake.mu.Lock(); fake.walletEmpty = false; fake.mu.Unlock() },
				Config:    config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "rep-1"),
					resource.TestCheckResourceAttr(name, "instance_class", "s-2vcpu-4gb"),
					resource.TestCheckResourceAttr(name, "status", "provisioning"),
					resource.TestCheckResourceAttr(name, "site", "hel1"),
				),
			},
			{ResourceName: name, ImportState: true, ImportStateId: "db-1:rep-1", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"accept_price_changes"}},
		},
	})
}
