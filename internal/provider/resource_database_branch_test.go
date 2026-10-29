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
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type databaseBranchTestProvider struct{ stackshiftProvider }

func (databaseBranchTestProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewDatabaseBranchResource, NewDatabaseResource}
}

func (databaseBranchTestProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewDatabaseCredentialsDataSource}
}

var databaseBranchFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"stackshift": providerserver.NewProtocol6WithError(&databaseBranchTestProvider{}),
}

type fakeBranchAPI struct {
	mu          sync.Mutex
	exists      bool
	gone        bool
	failCreate  bool
	priced      bool
	class       string
	attempts    []map[string]any
	createReads int
	deleteReads int
	deleteCalls int
	name        string
	state       string
	source      map[string]any
	sourceRef   any
	expiresAt   any
	sleep       map[string]any
	createBody  map[string]any
	patches     []map[string]any
	sleepPuts   []map[string]any
}

const fakeBranchExpiry = "2026-11-05T10:00:00Z"

func (f *fakeBranchAPI) branch() map[string]any {
	sleep := f.sleep
	if sleep == nil {
		sleep = map[string]any{"enabled": false, "after_minutes": 0, "state": "awake"}
	}
	branch := map[string]any{
		"id": "br-1", "name": f.name, "parent_database_id": "db-1", "root_database_id": "db-1", "depth": 1, "mechanism": "instant",
		"source": f.source, "state": f.state, "branched_at": "2026-10-29T10:00:00Z",
		"database": map[string]any{"id": "br-1", "private_hostname": "br-1.db.stackshift.internal", "sleep": sleep},
	}
	if f.priced {
		branch["database"].(map[string]any)["dedicated"] = map[string]any{"instance_class": f.class}
	}
	if f.sourceRef != nil {
		branch["source_ref"] = f.sourceRef
	}
	if f.expiresAt != nil {
		branch["expires_at"] = f.expiresAt
	}
	return branch
}

func (f *fakeBranchAPI) operation(id, kind, status string) map[string]any {
	op := map[string]any{"id": id, "branch_database_id": "br-1", "parent_database_id": "db-1", "kind": kind, "status": status}
	if kind == "create" {
		op["branch"] = f.branch()
	}
	return op
}

func (f *fakeBranchAPI) setSleep(minutes any) {
	f.sleep = map[string]any{"enabled": minutes != nil, "after_minutes": 0, "state": "awake"}
	if minutes != nil {
		f.sleep["after_minutes"] = minutes
	}
}

func (f *fakeBranchAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	route := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/v1")
	switch route {
	case "POST /databases/db-1/branches":
		f.attempts = append(f.attempts, body)
		if confirm, _ := body["confirm_charge_minor"].(float64); f.priced && int64(confirm) < fakeChargeMinor {
			writeJSON(w, http.StatusConflict, map[string]any{"success": false, "message": "This branch costs extra.",
				"error": map[string]any{"code": "price_confirmation_required"},
				"quote": map[string]any{"current_monthly_minor": 900000, "new_monthly_minor": 1050000, "charge_minor": fakeChargeMinor, "credit_minor": 0, "currency": "NGN", "billing": "monthly"}})
			return
		}
		f.exists, f.gone, f.createReads, f.createBody = true, false, 0, body
		f.class = "s-2vcpu-4gb"
		if requested, ok := body["instance_class"].(string); ok {
			f.class = requested
		}
		f.name, f.state, f.sleep, f.expiresAt, f.sourceRef = body["name"].(string), "creating", nil, nil, body["source_ref"]
		f.source = map[string]any{"kind": "now"}
		switch from := body["from"].(type) {
		case map[string]any:
			if value, ok := from["time"].(string); ok {
				moment, _ := time.Parse(time.RFC3339, value)
				f.source = map[string]any{"kind": "time", "time": moment.UTC().Format(time.RFC3339)}
			}
			if value, ok := from["restore_point_id"]; ok {
				f.source = map[string]any{"kind": "restore_point", "restore_point_id": value}
			}
			if value, ok := from["backup_id"]; ok {
				f.source = map[string]any{"kind": "backup", "backup_id": value}
			}
		}
		if body["ttl"] != nil {
			f.expiresAt = fakeBranchExpiry
		}
		if body["sleep_after_minutes"] != nil {
			f.setSleep(body["sleep_after_minutes"])
		}
		writeOK(w, http.StatusAccepted, f.operation("op-1", "create", "pending"))
	case "GET /databases/db-1/branches/operations/op-1":
		f.createReads++
		status := "running"
		switch {
		case f.createReads >= 2 && f.failCreate:
			f.state = "failed"
			op := f.operation("op-1", "create", "failed")
			op["error_code"], op["error_message"] = "no_capacity", "There is no room for another branch right now."
			writeOK(w, http.StatusOK, op)
			return
		case f.createReads >= 2:
			status, f.state = "succeeded", "ready"
		}
		writeOK(w, http.StatusOK, f.operation("op-1", "create", status))
	case "GET /databases/db-1/branches/operations/op-2":
		f.deleteReads++
		status := "running"
		if f.deleteReads >= 2 {
			status, f.gone = "succeeded", true
		}
		writeOK(w, http.StatusOK, f.operation("op-2", "delete", status))
	case "GET /databases/db-1/branches/br-1":
		if !f.exists || f.gone {
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "branch not found"})
			return
		}
		writeOK(w, http.StatusOK, f.branch())
	case "PATCH /databases/db-1/branches/br-1":
		f.patches = append(f.patches, body)
		if value, ok := body["name"].(string); ok {
			f.name = value
		}
		if value, ok := body["ttl"].(string); ok {
			f.expiresAt = fakeBranchExpiry
			if value == "never" {
				f.expiresAt = nil
			}
		}
		if value, ok := body["sleep_after_minutes"]; ok {
			f.setSleep(value)
		}
		writeOK(w, http.StatusOK, f.branch())
	case "PUT /databases/br-1/sleep":
		f.sleepPuts = append(f.sleepPuts, body)
		f.setSleep(nil)
		writeOK(w, http.StatusOK, f.sleep)
	case "DELETE /databases/db-1/branches/br-1":
		f.deleteCalls++
		f.deleteReads = 0
		writeOK(w, http.StatusAccepted, f.operation("op-2", "delete", "pending"))
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "no route " + route})
	}
}

func startFakeBranchAPI(t *testing.T, fake http.Handler) {
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("STACKSHIFT_ENDPOINT", server.URL)
	t.Setenv("STACKSHIFT_API_TOKEN", "sspat_test")
}

func branchConfig(extra string) string {
	return fmt.Sprintf(`
resource "stackshift_database_branch" "fix" {
  database_id = "db-1"
  name        = "fix-checkout"
%s
}
`, extra)
}

func branchDestroyed(fake *fakeBranchAPI) func(*terraform.State) error {
	return func(*terraform.State) error {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if fake.deleteCalls != 1 || !fake.gone || fake.deleteReads < 2 {
			return fmt.Errorf("delete calls = %d, gone = %v after %d reads", fake.deleteCalls, fake.gone, fake.deleteReads)
		}
		return nil
	}
}

func TestDatabaseBranchCreateUpdateImportAndDelete(t *testing.T) {
	fastPolling(t)
	fake := &fakeBranchAPI{}
	startFakeBranchAPI(t, fake)
	name := "stackshift_database_branch.fix"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		CheckDestroy:             branchDestroyed(fake),
		Steps: []resource.TestStep{
			{
				Config: branchConfig("  ttl                 = \"24h\"\n  sleep_after_minutes = 15\n  source_ref          = \"pr:142\""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "br-1"),
					resource.TestCheckResourceAttr(name, "name", "fix-checkout"),
					resource.TestCheckResourceAttr(name, "root_database_id", "db-1"),
					resource.TestCheckResourceAttr(name, "mechanism", "instant"),
					resource.TestCheckResourceAttr(name, "state", "ready"),
					resource.TestCheckResourceAttr(name, "private_hostname", "br-1.db.stackshift.internal"),
					resource.TestCheckResourceAttr(name, "branched_at", "2026-10-29T10:00:00Z"),
					resource.TestCheckResourceAttr(name, "expires_at", fakeBranchExpiry),
					resource.TestCheckResourceAttr(name, "from.now", "true"),
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "15"),
					resource.TestCheckResourceAttr(name, "source_ref", "pr:142"),
					func(*terraform.State) error {
						if fake.createReads < 2 || fake.createBody["from"] != "now" || fake.createBody["ttl"] != "24h" ||
							fake.createBody["sleep_after_minutes"] != float64(15) || fake.createBody["source_ref"] != "pr:142" {
							return fmt.Errorf("create body = %v after %d operation reads", fake.createBody, fake.createReads)
						}
						return nil
					},
				),
			},
			{
				Config: strings.Replace(branchConfig("  ttl                 = \"7d\"\n  sleep_after_minutes = 30\n  source_ref          = \"pr:142\""), "fix-checkout", "fix-checkout-2", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "br-1"),
					resource.TestCheckResourceAttr(name, "name", "fix-checkout-2"),
					resource.TestCheckResourceAttr(name, "ttl", "7d"),
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "30"),
					func(*terraform.State) error {
						want := `[map[name:fix-checkout-2 sleep_after_minutes:30 ttl:7d]]`
						if got := fmt.Sprint(fake.patches); got != want {
							return fmt.Errorf("patches = %s, want %s", got, want)
						}
						return nil
					},
				),
			},
			{
				Config: strings.Replace(branchConfig("  source_ref = \"pr:142\""), "fix-checkout", "fix-checkout-2", 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "ttl"),
					resource.TestCheckNoResourceAttr(name, "sleep_after_minutes"),
					resource.TestCheckNoResourceAttr(name, "expires_at"),
					func(*terraform.State) error {
						if len(fake.patches) != 2 || fake.patches[1]["ttl"] != "never" || len(fake.patches[1]) != 1 {
							return fmt.Errorf("patches = %v", fake.patches)
						}
						if len(fake.sleepPuts) != 1 || fake.sleepPuts[0]["enabled"] != false {
							return fmt.Errorf("sleep puts = %v", fake.sleepPuts)
						}
						return nil
					},
				),
			},
			{
				ResourceName: name, ImportState: true, ImportStateId: "db-1/br-1", ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{"ttl"},
			},
		},
	})
}

func TestDatabaseBranchOfDedicatedDatabaseNeedsAcceptedPrice(t *testing.T) {
	fastPolling(t)
	fake := &fakeBranchAPI{priced: true}
	startFakeBranchAPI(t, fake)
	name := "stackshift_database_branch.fix"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		CheckDestroy: func(*terraform.State) error {
			if !fake.gone || fake.deleteCalls != 2 {
				return fmt.Errorf("delete calls = %d, gone = %v", fake.deleteCalls, fake.gone)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config:      branchConfig("  instance_class = \"d-4vcpu-16gb\""),
				ExpectError: regexp.MustCompile(`(?s)new branch costs extra.*NGN 1500\.00.*NGN 10500\.00.*accept_price_changes = true`),
			},
			{
				Config: branchConfig("  instance_class = \"d-4vcpu-16gb\"\n  accept_price_changes = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "br-1"),
					resource.TestCheckResourceAttr(name, "state", "ready"),
					resource.TestCheckResourceAttr(name, "instance_class", "d-4vcpu-16gb"),
					resource.TestCheckResourceAttr(name, "accept_price_changes", "true"),
					func(*terraform.State) error {
						got := fmt.Sprint(fake.attempts)
						want := "[map[from:now instance_class:d-4vcpu-16gb name:fix-checkout] map[from:now instance_class:d-4vcpu-16gb name:fix-checkout] map[confirm_charge_minor:150000 from:now instance_class:d-4vcpu-16gb name:fix-checkout]]"
						if got != want {
							return fmt.Errorf("create attempts = %s, want %s", got, want)
						}
						return nil
					},
				),
			},
			{
				Config: branchConfig("  instance_class = \"s-2vcpu-4gb\"\n  accept_price_changes = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr(name, "instance_class", "s-2vcpu-4gb"),
			},
		},
	})
}

func TestDatabaseBranchImportRejectsMalformedID(t *testing.T) {
	startFakeBranchAPI(t, &fakeBranchAPI{})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{Config: branchConfig(""), ResourceName: "stackshift_database_branch.fix", ImportState: true, ImportStateId: "br-1", ExpectError: regexp.MustCompile(`parent_database_id/branch_id`)},
		},
	})
}

func TestDatabaseBranchRemovedWhenReadReturnsNotFound(t *testing.T) {
	fastPolling(t)
	fake := &fakeBranchAPI{}
	startFakeBranchAPI(t, fake)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{Config: branchConfig("")},
			{
				PreConfig:          func() { fake.mu.Lock(); fake.gone = true; fake.mu.Unlock() },
				Config:             branchConfig(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestDatabaseBranchFromChoicesAndReplacement(t *testing.T) {
	fastPolling(t)
	fake := &fakeBranchAPI{}
	startFakeBranchAPI(t, fake)
	name := "stackshift_database_branch.fix"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{
				Config: branchConfig("  from = { time = \"2026-10-28T11:00:00+01:00\" }"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "from.time", "2026-10-28T11:00:00+01:00"),
					func(*terraform.State) error {
						from, _ := fake.createBody["from"].(map[string]any)
						if from["time"] != "2026-10-28T11:00:00+01:00" {
							return fmt.Errorf("create body = %v", fake.createBody)
						}
						return nil
					},
				),
			},
			{
				Config: branchConfig("  from = { restore_point_id = \"rp-1\" }"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "from.restore_point_id", "rp-1"),
					func(*terraform.State) error {
						from, _ := fake.createBody["from"].(map[string]any)
						if from["restore_point_id"] != "rp-1" {
							return fmt.Errorf("create body = %v", fake.createBody)
						}
						return nil
					},
				),
			},
			{
				Config: branchConfig("  from = { backup_id = \"bk-1\" }"),
				Check:  resource.TestCheckResourceAttr(name, "from.backup_id", "bk-1"),
			},
		},
	})
}

func TestDatabaseBranchFailedCreateIsCleanedUpAndLeftOutOfState(t *testing.T) {
	fastPolling(t)
	fake := &fakeBranchAPI{failCreate: true}
	startFakeBranchAPI(t, fake)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		CheckDestroy:             branchDestroyed(fake),
		Steps: []resource.TestStep{
			{Config: branchConfig(""), ExpectError: regexp.MustCompile(`There is no room for another branch right now`)},
		},
	})
}

func TestDatabaseBranchRejectsInvalidSettings(t *testing.T) {
	startFakeBranchAPI(t, &fakeBranchAPI{})
	invalid := func(extra, message string) resource.TestStep {
		return resource.TestStep{Config: branchConfig(extra), PlanOnly: true, ExpectError: regexp.MustCompile(message)}
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			invalid("  from = { now = true, backup_id = \"bk-1\" }", `Choose where the branch starts from`),
			invalid("  from = {}", `Choose where the branch starts from`),
			invalid("  from = { time = \"yesterday\" }", `Use an RFC 3339 time`),
			invalid("  ttl = \"soon\"", `hours or days`),
			invalid("  sleep_after_minutes = 3", `Choose 5 to 1440 minutes`),
		},
	})
}

type fakeStarterAPI struct {
	mu          sync.Mutex
	exists      bool
	refuseSleep bool
	engine      string
	sleep       map[string]any
	sleepPuts   []map[string]any
}

func (f *fakeStarterAPI) database() map[string]any {
	engine := f.engine
	if engine == "" {
		engine = "postgres"
	}
	database := map[string]any{"id": "db-s1", "project_id": "p-1", "name": "orders", "type": engine, "version": "16", "host": "db-s1.stackshift.internal",
		"port": 5432, "tls_mode": "require", "database_name": "app", "size_gb": 1, "status": "running", "created_at": "2026-10-29T10:00:00Z"}
	if f.sleep != nil && engine == "postgres" {
		database["sleep"] = f.sleep
	}
	return database
}

func (f *fakeStarterAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	route := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/v1")
	switch route {
	case "POST /projects/p-1/databases":
		f.exists, f.sleep = true, nil
		f.engine = body["type"].(string)
		writeOK(w, http.StatusCreated, map[string]any{"database": f.database()})
	case "GET /databases/db-s1":
		if !f.exists {
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "database not found"})
			return
		}
		writeOK(w, http.StatusOK, f.database())
	case "PUT /databases/db-s1/sleep":
		f.sleepPuts = append(f.sleepPuts, body)
		if f.refuseSleep {
			writeJSON(w, http.StatusConflict, map[string]any{"success": false, "message": "This database is not placed on a server yet.",
				"error": map[string]any{"code": "sleep_unavailable"}})
			return
		}
		f.sleep = map[string]any{"enabled": body["enabled"] == true, "after_minutes": 5, "state": "awake"}
		if body["enabled"] == true {
			f.sleep["after_minutes"] = body["after_minutes"]
		}
		writeOK(w, http.StatusOK, f.sleep)
	case "DELETE /projects/p-1/databases/db-s1":
		f.exists = false
		writeOK(w, http.StatusOK, map[string]any{"message": "deleted"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": "no route " + route})
	}
}

func starterDatabaseConfig(engine, extra string) string {
	return fmt.Sprintf(`
resource "stackshift_database" "orders" {
  project_id = "p-1"
  name       = "orders"
  type       = %q
  version    = "16"
  size_gb    = 1
%s
}
`, engine, extra)
}

func TestDatabaseSleepWhenIdleTurnsOnChangesAndOff(t *testing.T) {
	fake := &fakeStarterAPI{}
	startFakeBranchAPI(t, fake)
	name := "stackshift_database.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{
				Config: starterDatabaseConfig("postgres", "  sleep_after_minutes = 15"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-s1"),
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "15"),
					func(*terraform.State) error {
						if fmt.Sprint(fake.sleepPuts) != "[map[after_minutes:15 enabled:true]]" {
							return fmt.Errorf("sleep puts = %v", fake.sleepPuts)
						}
						return nil
					},
				),
			},
			{
				Config: starterDatabaseConfig("postgres", "  sleep_after_minutes = 30"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-s1"),
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "30"),
					resource.TestCheckResourceAttr(name, "status", "running"),
				),
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true},
			{
				Config: starterDatabaseConfig("postgres", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "sleep_after_minutes"),
					func(*terraform.State) error {
						if len(fake.sleepPuts) != 3 || fake.sleepPuts[2]["enabled"] != false || fake.sleepPuts[2]["after_minutes"] != nil {
							return fmt.Errorf("sleep puts = %v", fake.sleepPuts)
						}
						return nil
					},
				),
			},
			{
				Config: starterDatabaseConfig("postgres", "  sleep_after_minutes = 0"),
				Check:  resource.TestCheckResourceAttr(name, "sleep_after_minutes", "0"),
			},
		},
	})
}

func TestDatabaseSleepWhenIdleRefusedOnCreateKeepsTheDatabase(t *testing.T) {
	fake := &fakeStarterAPI{refuseSleep: true}
	startFakeBranchAPI(t, fake)
	name := "stackshift_database.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{
				Config:             starterDatabaseConfig("postgres", "  sleep_after_minutes = 15"),
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-s1"),
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "15"),
				),
			},
			{
				PreConfig: func() { fake.mu.Lock(); fake.refuseSleep = false; fake.mu.Unlock() },
				Config:    starterDatabaseConfig("postgres", "  sleep_after_minutes = 15"),
				Check:     resource.TestCheckResourceAttr(name, "sleep_after_minutes", "15"),
			},
		},
	})
}

func TestDatabaseSleepWhenIdleIsOnlyForPostgres(t *testing.T) {
	startFakeBranchAPI(t, &fakeStarterAPI{})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: databaseBranchFactories,
		Steps: []resource.TestStep{
			{Config: starterDatabaseConfig("mysql", "  sleep_after_minutes = 15"), PlanOnly: true, ExpectError: regexp.MustCompile(`only for PostgreSQL`)},
			{Config: starterDatabaseConfig("postgres", "  sleep_after_minutes = 2"), PlanOnly: true, ExpectError: regexp.MustCompile(`Choose 5 to 1440 minutes`)},
		},
	})
}

func TestDedicatedDatabaseSleepWhenIdleTurnsOnAndOff(t *testing.T) {
	fastPolling(t)
	fake := &fakeDedicatedAPI{}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{
				Config: dedicatedDatabaseConfig("s-2vcpu-4gb", "  sleep_after_minutes = 15"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "sleep_after_minutes", "15"),
					func(*terraform.State) error {
						if fmt.Sprint(fake.sleepPuts) != "[map[after_minutes:15 enabled:true]]" {
							return fmt.Errorf("sleep puts = %v", fake.sleepPuts)
						}
						return nil
					},
				),
			},
			{
				Config: dedicatedDatabaseConfig("s-2vcpu-4gb", "  sleep_after_minutes = 60"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr(name, "sleep_after_minutes", "60"),
			},
			{
				Config: dedicatedDatabaseConfig("s-2vcpu-4gb", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "sleep_after_minutes"),
					func(*terraform.State) error {
						if len(fake.sleepPuts) != 3 || fake.sleepPuts[2]["enabled"] != false {
							return fmt.Errorf("sleep puts = %v", fake.sleepPuts)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestDedicatedDatabaseSleepWhenIdleIsOnlyForPostgres(t *testing.T) {
	startFakeDedicatedAPI(t, &fakeDedicatedAPI{})
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{Config: dedicatedDatabaseConfig("s-2vcpu-4gb", "  engine = \"mysql\"\n  sleep_after_minutes = 15"), PlanOnly: true, ExpectError: regexp.MustCompile(`only for PostgreSQL`)},
		},
	})
}
