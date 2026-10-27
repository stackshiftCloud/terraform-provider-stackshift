package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type managementState struct {
	group          map[string]any
	groupAttached  string
	restarts       int
	snapshot       map[string]any
	snapshotReads  int
	restorePoint   map[string]any
	pointReads     int
	schedules      []map[string]any
	scheduleBodies []map[string]any
	policy         map[string]any
	policyReads    int
	policyRevoked  bool
	policyBody     map[string]any
	insightsDays   float64
	restartReads   int
	revokedPolicy  string
}

func fastPolling(t *testing.T) {
	previous := dedicatedPollInterval
	dedicatedPollInterval = time.Millisecond
	t.Cleanup(func() { dedicatedPollInterval = previous })
}

func runningDedicatedAPI() *fakeDedicatedAPI {
	return &fakeDedicatedAPI{created: true, status: "running", class: "s-2vcpu-4gb", settings: map[string]any{
		"storage_gb": 20, "storage_max_gb": 40, "backup_retention_days": 14, "deletion_protection": true,
		"maintenance_window": map[string]any{"day_of_week": 2, "start_minute": 120}, "billing": "monthly", "standby": "sync", "on_standby_loss": "block",
	}}
}

func (m *managementState) defaultSchedules() []map[string]any {
	if m.schedules == nil {
		m.schedules = []map[string]any{
			{"kind": "daily", "enabled": true, "keep": float64(7)},
			{"kind": "weekly", "enabled": true, "keep": float64(4)},
			{"kind": "monthly", "enabled": true, "keep": float64(3)},
		}
	}
	return m.schedules
}

func (m *managementState) parametersView() map[string]any {
	view := map[string]any{"family": "postgres17", "applied": true, "pending_restart": []string{}}
	if m.groupAttached != "" {
		view["group"] = map[string]any{"id": m.groupAttached, "name": "tuned", "version": 1}
		if m.restarts == 0 || m.restartReads < 2 {
			view["pending_restart"], view["applied"] = []string{"shared_buffers"}, false
		}
	}
	return view
}

func (m *managementState) serve(f *fakeDedicatedAPI, w http.ResponseWriter, route, family string, body map[string]any) bool {
	notFoundJSON := func(message string) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "message": message})
	}
	switch route {
	case "POST /databases/parameter-groups":
		m.group = map[string]any{"id": "grp-1", "name": body["name"], "family": body["family"], "description": body["description"], "version": float64(1),
			"parameters": body["parameters"], "databases": float64(0)}
		writeOK(w, http.StatusCreated, m.group)
	case "GET /databases/parameter-groups/grp-1":
		if m.group == nil {
			notFoundJSON("parameter group not found")
			return true
		}
		writeOK(w, http.StatusOK, m.group)
	case "PATCH /databases/parameter-groups/grp-1":
		for _, key := range []string{"name", "description", "parameters"} {
			m.group[key] = body[key]
		}
		m.group["version"] = m.group["version"].(float64) + 1
		writeOK(w, http.StatusOK, m.group)
	case "DELETE /databases/parameter-groups/grp-1":
		m.group = nil
		writeOK(w, http.StatusOK, map[string]any{"deleted": true})
	case "GET /databases/parameter-definitions":
		writeOK(w, http.StatusOK, map[string]any{"parameters": []map[string]any{
			{"name": "work_mem", "kind": "memory", "min": 64, "max": 2097152, "restart": false, "description": "Memory per sort for " + family},
			{"name": "shared_buffers", "kind": "memory", "min": 16384, "restart": true, "engine_default": "128MB", "description": "Page cache"},
		}})
	case "PUT /databases/db-1/parameter-group":
		m.groupAttached = ""
		if id, ok := body["group_id"].(string); ok {
			m.groupAttached = id
		}
		writeOK(w, http.StatusOK, m.parametersView())
	case "GET /databases/db-1/parameters":
		if m.restarts > 0 {
			m.restartReads++
		}
		writeOK(w, http.StatusOK, m.parametersView())
	case "POST /databases/db-1/parameters/apply":
		m.restarts++
		writeOK(w, http.StatusOK, m.parametersView())
	case "GET /databases/db-1/backups":
		backups := []map[string]any{{"id": "bk-1", "kind": "scheduled", "repo": "pitr", "status": "available", "size_bytes": 2048,
			"started_at": "2026-10-25T01:00:00Z", "finished_at": "2026-10-25T01:05:00Z"}}
		if m.snapshot != nil {
			backups = append(backups, m.snapshot)
		}
		writeOK(w, http.StatusOK, map[string]any{"retention_days": 14, "restorable": map[string]any{"earliest": "2026-10-20T00:00:00Z", "latest": "2026-10-26T09:00:00Z"}, "backups": backups})
	case "POST /databases/db-1/backups":
		m.snapshot = map[string]any{"id": "snap-1", "kind": "manual", "repo": "snapshots", "name": body["name"], "status": "creating", "size_bytes": 0,
			"created_at": "2026-10-26T10:00:00Z"}
		writeOK(w, http.StatusAccepted, map[string]any{"backup": m.snapshot})
	case "GET /databases/db-1/backups/snap-1":
		if m.snapshot == nil {
			notFoundJSON("backup not found")
			return true
		}
		if m.snapshotReads++; m.snapshotReads >= 2 {
			m.snapshot["status"], m.snapshot["size_bytes"] = "available", 1024
		}
		writeOK(w, http.StatusOK, m.snapshot)
	case "DELETE /databases/db-1/backups/snap-1":
		m.snapshot = nil
		writeOK(w, http.StatusAccepted, map[string]any{"message": "deleting"})
	case "GET /databases/db-1/restore-points":
		points := []map[string]any{}
		if m.restorePoint != nil {
			if m.pointReads++; m.pointReads >= 2 {
				m.restorePoint["status"], m.restorePoint["taken_at"] = "available", "2026-10-26T10:01:00Z"
			}
			points = append(points, m.restorePoint)
		}
		writeOK(w, http.StatusOK, map[string]any{"eligibility": map[string]any{"available": true}, "schedules": m.defaultSchedules(), "restore_points": points})
	case "POST /databases/db-1/restore-points":
		m.restorePoint = map[string]any{"id": "rp-1", "kind": "manual", "name": body["name"], "status": "creating", "size_gb": 20}
		writeOK(w, http.StatusAccepted, map[string]any{"restore_point": m.restorePoint})
	case "DELETE /databases/db-1/restore-points/rp-1":
		m.restorePoint = nil
		writeOK(w, http.StatusAccepted, map[string]any{"message": "deleting"})
	case "PUT /databases/db-1/restore-points/schedules":
		m.scheduleBodies = append(m.scheduleBodies, body)
		for _, schedule := range m.defaultSchedules() {
			if change, ok := body[schedule["kind"].(string)].(map[string]any); ok {
				schedule["enabled"], schedule["keep"] = change["enabled"], change["keep"]
			}
		}
		writeOK(w, http.StatusOK, map[string]any{"schedules": m.schedules})
	case "POST /databases/db-1/public-access":
		m.policyBody, m.policyRevoked, m.policyReads = body, false, 0
		expires := "2026-11-02T00:00:00Z"
		if requested, ok := body["expires_at"].(string); ok {
			expires = requested
		}
		m.policy = map[string]any{"id": "pol-1", "transport": "tunnel", "access_mode": body["access_mode"], "status": "pending", "allowed_cidrs": body["allowed_cidrs"],
			"expires_at": expires, "public_hostname": "tunnel.example.test", "public_port": 15432}
		writeOK(w, http.StatusAccepted, map[string]any{"policy": m.policy, "created": true})
	case "GET /databases/db-1/public-access/pol-1":
		if m.policy == nil {
			notFoundJSON("database public access resource not found")
			return true
		}
		switch {
		case m.policyRevoked:
			m.policy["status"] = "revoked"
		case m.policyReads == 0:
			m.policy["status"] = "applying"
		default:
			m.policy["status"] = "active"
		}
		m.policyReads++
		writeOK(w, http.StatusOK, map[string]any{"policy": m.policy})
	case "POST /databases/db-1/public-access/revoke":
		m.revokedPolicy, _ = body["policy_id"].(string)
		if m.policy == nil || m.revokedPolicy != "pol-1" || m.policyRevoked {
			notFoundJSON("database public access resource not found")
			return true
		}
		m.policyRevoked = true
		writeOK(w, http.StatusAccepted, map[string]any{"policy": m.policy})
	case "GET /databases/src-2/backups":
		writeOK(w, http.StatusOK, map[string]any{"retention_days": 7, "restorable": map[string]any{}, "backups": []map[string]any{}})
	case "GET /databases/db-1/insights":
		days := m.insightsDays
		if days == 0 {
			days = 7
		}
		writeOK(w, http.StatusOK, map[string]any{"retention_days": days, "included_days": 7})
	case "PUT /databases/db-1/insights":
		days := body["retention_days"].(float64)
		if days != 7 && !f.paid(w, body, "insights") {
			return true
		}
		m.insightsDays = days
		writeOK(w, http.StatusOK, map[string]any{"retention_days": days, "included_days": 7})
	case "GET /databases/db-1/credentials":
		writeOK(w, http.StatusOK, map[string]any{"username": "app_owner", "password": "pw", "database_name": "app", "host": "db-1.stackshift.internal", "port": 5432,
			"tls_mode": "require", "tls_server_name": "db-1.stackshift.internal", "ca_certificate": "-----BEGIN CERTIFICATE-----", "connection_string": "postgresql://primary",
			"endpoints": []map[string]any{
				{"protocol": "postgresql", "network": "private", "role": "direct", "primary": true, "host": "db-1.stackshift.internal", "port": 5432, "tls_mode": "verify-full", "connection_string": "postgresql://primary"},
				{"protocol": "postgresql", "network": "private", "role": "read", "primary": false, "host": "db-1-ro.stackshift.internal", "port": 5432, "tls_mode": "verify-full", "connection_string": "postgresql://read", "ca_filename": "ca.pem"},
			}})
	default:
		return false
	}
	return true
}

func TestDedicatedDatabaseMySQLRenameAndBilling(t *testing.T) {
	fastPolling(t)
	fake := &fakeDedicatedAPI{}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.orders"
	config := func(dbName, engine, extra string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database" "orders" {
  name           = %q
  engine         = %q
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20
  maintenance_window = {
    day_of_week  = 2
    start_minute = 120
  }
%s
}
`, dbName, engine, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{
				Config: config("orders", "mysql", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "engine", "mysql"),
					resource.TestCheckResourceAttr(name, "version", "8.4"),
					resource.TestCheckResourceAttr(name, "backup_retention_days", "7"),
					resource.TestCheckResourceAttr(name, "master_password", "s3cret-once"),
					resource.TestCheckResourceAttr(name, "endpoints.0.protocol", "mysql"),
					func(*terraform.State) error {
						if fake.createBody["type"] != "mysql" || fake.createBody["version"] != nil || fake.createBody["master_username"] != nil || fake.createBody["database_name"] != nil {
							return fmt.Errorf("create body = %v", fake.createBody)
						}
						return nil
					},
				),
			},
			{
				Config: config("orders-v2", "mysql", `  billing = "yearly"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "name", "orders-v2"),
					resource.TestCheckResourceAttr(name, "billing", "yearly"),
					func(*terraform.State) error {
						if len(fake.settingsPatches) != 1 || fmt.Sprint(fake.settingsPatches[0]) != "map[name:orders-v2]" ||
							len(fake.billingPuts) != 1 || fake.billingPuts[0]["billing"] != "yearly" {
							return fmt.Errorf("patches = %v, billing = %v", fake.settingsPatches, fake.billingPuts)
						}
						return nil
					},
				),
			},
			{
				Config:           config("orders-v2", "mysql", `  billing = "yearly"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"master_password"}},
		},
	})
}

func TestDedicatedDatabaseRestoreFromSource(t *testing.T) {
	fastPolling(t)
	fake := runningDedicatedAPI()
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.restored"
	config := func(restore, extra string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database" "restored" {
  name           = "orders-restored"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20
%s
%s
}
`, restore, extra)
	}
	restoreBlock := func(attributes string) string {
		return "  restore_from = {\n    source_database_id = \"src-1\"\n" + attributes + "  }"
	}
	const restoreTime = "2026-10-25T14:30:00Z"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "stackshift_dedicated_database_backups" "empty" {
  database_id = "src-2"
  depends_on  = [stackshift_dedicated_database_snapshot.gate]
}

resource "stackshift_dedicated_database_snapshot" "gate" {
  database_id = "db-1"
}

resource "stackshift_dedicated_database" "restored" {
  name           = "orders-restored"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20

  restore_from = {
    source_database_id = "src-2"
    target_time        = data.stackshift_dedicated_database_backups.empty.restorable_until
  }
}
`,
				ExpectError: regexp.MustCompile(`Set\s+exactly\s+one\s+of\s+target_time`),
			},
			{
				Config:      config(restoreBlock("    backup_id   = \"bk-1\"\n    target_time = \""+restoreTime+"\"\n"), ""),
				ExpectError: regexp.MustCompile(`Set\s+exactly\s+one\s+of\s+target_time`),
			},
			{
				Config:      config(restoreBlock("    backup_id = \"\"\n"), ""),
				ExpectError: regexp.MustCompile(`backup_id\s+can't\s+be\s+an\s+empty\s+string`),
			},
			{
				Config:      config(restoreBlock("    target_time = \"\"\n"), ""),
				ExpectError: regexp.MustCompile(`target_time\s+can't\s+be\s+an\s+empty\s+string`),
			},
			{
				Config:      config(restoreBlock("    target_time = \"yesterday\"\n"), ""),
				ExpectError: regexp.MustCompile(`Use\s+an\s+RFC\s+3339\s+time`),
			},
			{
				Config:      config(restoreBlock("    target_time = \""+restoreTime+"\"\n"), "  backup_retention_days = 10\n  standby = \"sync\""),
				ExpectError: regexp.MustCompile(`backup_retention_days,\s+standby\s+can't\s+be\s+set`),
			},
			{
				Config: config(restoreBlock("    target_time = \""+restoreTime+"\"\n"), ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "engine", "mysql"),
					resource.TestCheckResourceAttr(name, "version", "8.4"),
					resource.TestCheckResourceAttr(name, "backup_retention_days", "14"),
					resource.TestCheckResourceAttr(name, "storage_max_gb", "80"),
					resource.TestCheckResourceAttr(name, "maintenance_window.day_of_week", "1"),
					resource.TestCheckResourceAttr(name, "master_password", "restored-once"),
					resource.TestCheckResourceAttr(name, "restore_from.source_database_id", "src-1"),
					func(*terraform.State) error {
						if fake.createBody != nil || fake.restoreBody["target_time"] != restoreTime || fake.restoreBody["name"] != "orders-restored" ||
							fake.restoreBody["storage_gb"] != float64(20) || fake.restoreBody["backup_id"] != nil || fake.restoreBody["billing"] != "monthly" {
							return fmt.Errorf("create body = %v, restore body = %v", fake.createBody, fake.restoreBody)
						}
						return nil
					},
				),
			},
			{
				Config:           config("", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config:           config(restoreBlock("    target_time = \"2026-10-24T09:00:00Z\"\n"), ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func TestDatabaseParameterGroupAndAttachment(t *testing.T) {
	fastPolling(t)
	fake := runningDedicatedAPI()
	startFakeDedicatedAPI(t, fake)
	group, attachment := "stackshift_database_parameter_group.tuned", "stackshift_dedicated_database_parameter_group_attachment.tuned"
	config := func(family, memory string, restart bool) string {
		return fmt.Sprintf(`
resource "stackshift_database_parameter_group" "tuned" {
  name        = "tuned"
  family      = %q
  description = "Memory tuning"
  parameters = {
    work_mem = %q
  }
}

resource "stackshift_dedicated_database_parameter_group_attachment" "tuned" {
  database_id        = "db-1"
  parameter_group_id = stackshift_database_parameter_group.tuned.id
  restart_to_apply   = %t
}

data "stackshift_database_parameter_definitions" "postgres" {
  family = "postgres17"
}
`, family, memory, restart)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if fake.management.group != nil || fake.management.groupAttached != "" {
				return fmt.Errorf("group = %v, attached = %q", fake.management.group, fake.management.groupAttached)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config("postgres17", "65536", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(group, "id", "grp-1"),
					resource.TestCheckResourceAttr(group, "version", "1"),
					resource.TestCheckResourceAttr(group, "parameters.work_mem", "65536"),
					resource.TestCheckResourceAttr(attachment, "id", "db-1"),
					resource.TestCheckResourceAttr(attachment, "parameter_group_id", "grp-1"),
					resource.TestCheckResourceAttr(attachment, "applied", "false"),
					resource.TestCheckResourceAttr(attachment, "pending_restart.#", "1"),
					resource.TestCheckResourceAttr("data.stackshift_database_parameter_definitions.postgres", "parameters.#", "2"),
					resource.TestCheckResourceAttr("data.stackshift_database_parameter_definitions.postgres", "parameters.0.max", "2097152"),
					resource.TestCheckResourceAttr("data.stackshift_database_parameter_definitions.postgres", "parameters.0.description", "Memory per sort for postgres17"),
					resource.TestCheckResourceAttr("data.stackshift_database_parameter_definitions.postgres", "parameters.1.needs_restart", "true"),
					resource.TestCheckResourceAttr("data.stackshift_database_parameter_definitions.postgres", "parameters.1.engine_default", "128MB"),
					func(*terraform.State) error {
						if fake.management.groupAttached != "grp-1" || fake.management.restarts != 0 {
							return fmt.Errorf("attached = %q, restarts = %d", fake.management.groupAttached, fake.management.restarts)
						}
						return nil
					},
				),
			},
			{
				Config: config("postgres17", "65536", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(attachment, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(group, plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attachment, "applied", "true"),
					resource.TestCheckResourceAttr(attachment, "pending_restart.#", "0"),
					func(*terraform.State) error {
						if fake.management.restarts != 1 || fake.management.restartReads < 2 {
							return fmt.Errorf("restarts = %d, reads = %d", fake.management.restarts, fake.management.restartReads)
						}
						return nil
					},
				),
			},
			{
				Config: config("postgres17", "131072", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(group, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(attachment, plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(group, "version", "2"),
					resource.TestCheckResourceAttr(group, "parameters.work_mem", "131072"),
				),
			},
			{ResourceName: group, ImportState: true, ImportStateVerify: true},
			{ResourceName: attachment, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"restart_to_apply"}},
		},
	})
}

func TestDedicatedDatabaseSnapshotRestorePointAndSchedule(t *testing.T) {
	fastPolling(t)
	fake := runningDedicatedAPI()
	startFakeDedicatedAPI(t, fake)
	snapshot, point, schedule := "stackshift_dedicated_database_snapshot.before", "stackshift_dedicated_database_restore_point.nightly", "stackshift_dedicated_database_restore_schedule.plan"
	config := func(daily int, weekly string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database_snapshot" "before" {
  database_id = "db-1"
  name        = "before-migration"
}

resource "stackshift_dedicated_database_restore_point" "nightly" {
  database_id = "db-1"
  name        = "nightly"
}

resource "stackshift_dedicated_database_restore_schedule" "plan" {
  database_id = "db-1"
  daily = {
    enabled = true
    keep    = %d
  }
%s
}

data "stackshift_dedicated_database_backups" "orders" {
  database_id = "db-1"
  depends_on  = [stackshift_dedicated_database_snapshot.before]
}
`, daily, weekly)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if fake.management.snapshot != nil || fake.management.restorePoint != nil {
				return fmt.Errorf("snapshot = %v, restore point = %v", fake.management.snapshot, fake.management.restorePoint)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config(40, ""), ExpectError: regexp.MustCompile(`Keep\s+between\s+1\s+and\s+30`)},
			{
				Config: config(10, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(snapshot, "id", "snap-1"),
					resource.TestCheckResourceAttr(snapshot, "kind", "manual"),
					resource.TestCheckResourceAttr(snapshot, "status", "available"),
					resource.TestCheckResourceAttr(snapshot, "size_bytes", "1024"),
					resource.TestCheckResourceAttr(point, "id", "rp-1"),
					resource.TestCheckResourceAttr(point, "status", "available"),
					resource.TestCheckResourceAttr(point, "size_gb", "20"),
					resource.TestCheckResourceAttr(schedule, "id", "db-1"),
					resource.TestCheckResourceAttr(schedule, "daily.keep", "10"),
					resource.TestCheckResourceAttr(schedule, "weekly.keep", "4"),
					resource.TestCheckResourceAttr(schedule, "monthly.enabled", "true"),
					resource.TestCheckResourceAttr("data.stackshift_dedicated_database_backups.orders", "retention_days", "14"),
					resource.TestCheckResourceAttr("data.stackshift_dedicated_database_backups.orders", "restorable_from", "2026-10-20T00:00:00Z"),
					resource.TestCheckResourceAttr("data.stackshift_dedicated_database_backups.orders", "restorable_until", "2026-10-26T09:00:00Z"),
					resource.TestCheckResourceAttr("data.stackshift_dedicated_database_backups.orders", "backups.#", "2"),
					resource.TestCheckResourceAttr("data.stackshift_dedicated_database_backups.orders", "backups.1.name", "before-migration"),
					func(*terraform.State) error {
						if len(fake.management.scheduleBodies) != 1 || fmt.Sprint(fake.management.scheduleBodies[0]) != "map[daily:map[enabled:true keep:10]]" {
							return fmt.Errorf("schedule bodies = %v", fake.management.scheduleBodies)
						}
						return nil
					},
				),
			},
			{
				Config: config(12, "  weekly = {\n    enabled = false\n    keep    = 4\n  }"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(schedule, plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction(snapshot, plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(schedule, "daily.keep", "12"),
					resource.TestCheckResourceAttr(schedule, "weekly.enabled", "false"),
					func(*terraform.State) error {
						if len(fake.management.scheduleBodies) != 2 || fmt.Sprint(fake.management.scheduleBodies[1]) != "map[daily:map[enabled:true keep:12] weekly:map[enabled:false keep:4]]" {
							return fmt.Errorf("schedule bodies = %v", fake.management.scheduleBodies)
						}
						return nil
					},
				),
			},
			{ResourceName: snapshot, ImportState: true, ImportStateId: "db-1:snap-1", ImportStateVerify: true},
			{ResourceName: point, ImportState: true, ImportStateId: "db-1:rp-1", ImportStateVerify: true},
			{ResourceName: schedule, ImportState: true, ImportStateId: "db-1", ImportStateVerify: true},
		},
	})
}

func TestDedicatedDatabasePublicAccess(t *testing.T) {
	fastPolling(t)
	fake := runningDedicatedAPI()
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database_public_access.office"
	config := func(body string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database_public_access" "office" {
  database_id = "db-1"
%s
}
`, body)
	}
	restricted := `  access_mode   = "restricted"
  allowed_cidrs = ["203.0.113.0/24"]
  expires_at    = "2026-12-01T00:00:00Z"`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if !fake.management.policyRevoked || fake.management.revokedPolicy != "pol-1" {
				return fmt.Errorf("public access was not revoked: %q", fake.management.revokedPolicy)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config(`  access_mode = "restricted"`), ExpectError: regexp.MustCompile(`Set\s+allowed_cidrs\s+and\s+expires_at`)},
			{Config: config(`  access_mode = "anywhere"`), ExpectError: regexp.MustCompile(`acknowledge_public_exposure\s+=\s+true`)},
			{Config: config(strings.Replace(restricted, "2026-12-01T00:00:00Z", "2026-12-01", 1)), ExpectError: regexp.MustCompile(`Use\s+a\s+UTC\s+time`)},
			{Config: config(strings.Replace(restricted, "203.0.113.0/24", "203.0.113.5/24", 1)), ExpectError: regexp.MustCompile(`Use\s+canonical\s+address\s+ranges`)},
			{
				Config: config(restricted),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "pol-1"),
					resource.TestCheckResourceAttr(name, "status", "active"),
					resource.TestCheckResourceAttr(name, "public_hostname", "tunnel.example.test"),
					resource.TestCheckResourceAttr(name, "public_port", "15432"),
					resource.TestCheckResourceAttr(name, "expires_at", "2026-12-01T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "allowed_cidrs.#", "1"),
					func(*terraform.State) error {
						if fake.management.policyBody["transport"] != "tunnel" || fake.management.policyBody["access_mode"] != "restricted" ||
							fake.management.policyBody["acknowledge_public_exposure"] != nil || fmt.Sprint(fake.management.policyBody["allowed_cidrs"]) != "[203.0.113.0/24]" {
							return fmt.Errorf("policy body = %v", fake.management.policyBody)
						}
						return nil
					},
				),
			},
			{ResourceName: name, ImportState: true, ImportStateId: "db-1:pol-1", ImportStateVerify: true},
			{
				Config: config(`  access_mode                 = "anywhere"
  acknowledge_public_exposure = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionReplace)}},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "access_mode", "anywhere"),
					resource.TestCheckResourceAttr(name, "expires_at", "2026-11-02T00:00:00Z"),
					resource.TestCheckResourceAttr(name, "allowed_cidrs.#", "0"),
					func(*terraform.State) error {
						if fake.management.policyBody["acknowledge_public_exposure"] != true || fake.management.policyBody["allowed_cidrs"] != nil {
							return fmt.Errorf("policy body = %v", fake.management.policyBody)
						}
						return nil
					},
				),
			},
			{ResourceName: name, ImportState: true, ImportStateId: "db-1:pol-1", ImportStateVerify: true},
		},
	})
}

func TestDedicatedDatabaseInsights(t *testing.T) {
	fake := runningDedicatedAPI()
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database_insights.history"
	config := func(days int, extra string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database_insights" "history" {
  database_id    = "db-1"
  retention_days = %d
%s
}
`, days, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if fake.management.insightsDays != 7 {
				return fmt.Errorf("retention after destroy = %v", fake.management.insightsDays)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config(14, ""), ExpectError: regexp.MustCompile(`one\s+of\s+\[7\s+30\s+90\]`)},
			{Config: config(30, ""), ExpectError: regexp.MustCompile(`(?s)costs extra.*NGN 1500\.00.*accept_price_changes = true`)},
			{
				Config: config(30, "  accept_price_changes = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", "db-1"),
					resource.TestCheckResourceAttr(name, "retention_days", "30"),
					resource.TestCheckResourceAttr(name, "included_days", "7"),
					func(*terraform.State) error {
						if fake.management.insightsDays != 30 || strings.Join(fake.confirms, " ") != "insights:<nil> insights:<nil> insights:150000" {
							return fmt.Errorf("days = %v, confirms = %v", fake.management.insightsDays, fake.confirms)
						}
						return nil
					},
				),
			},
			{ResourceName: name, ImportState: true, ImportStateId: "db-1", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"accept_price_changes"}},
		},
	})
}

func TestDedicatedDatabaseDataSourceAndCredentials(t *testing.T) {
	fake := runningDedicatedAPI()
	fake.replicas = []map[string]any{{"id": "rep-1", "role": "replica", "instance_class": "s-2vcpu-4gb", "status": "running", "site": "hel1"}}
	startFakeDedicatedAPI(t, fake)
	database, credentials := "data.stackshift_dedicated_database.orders", "data.stackshift_database_credentials.orders"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{{
			Config: `
data "stackshift_dedicated_database" "orders" {
  id = "db-1"
}

data "stackshift_database_credentials" "orders" {
  database_id = "db-1"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(database, "name", "orders"),
				resource.TestCheckResourceAttr(database, "engine", "postgres"),
				resource.TestCheckResourceAttr(database, "version", "17"),
				resource.TestCheckResourceAttr(database, "status", "running"),
				resource.TestCheckResourceAttr(database, "instance_class", "s-2vcpu-4gb"),
				resource.TestCheckResourceAttr(database, "allocated_storage_gb", "20"),
				resource.TestCheckResourceAttr(database, "storage_max_gb", "40"),
				resource.TestCheckResourceAttr(database, "backup_retention_days", "14"),
				resource.TestCheckResourceAttr(database, "deletion_protection", "true"),
				resource.TestCheckResourceAttr(database, "maintenance_window.start_minute", "120"),
				resource.TestCheckResourceAttr(database, "standby", "off"),
				resource.TestCheckResourceAttr(database, "billing", "monthly"),
				resource.TestCheckResourceAttr(database, "billing_status", "active"),
				resource.TestCheckResourceAttr(database, "tls_mode", "require"),
				resource.TestCheckResourceAttr(database, "endpoints.0.role", "primary"),
				resource.TestCheckResourceAttr(database, "members.#", "2"),
				resource.TestCheckResourceAttr(database, "members.1.role", "replica"),
				resource.TestCheckResourceAttr(credentials, "ca_certificate", "-----BEGIN CERTIFICATE-----"),
				resource.TestCheckResourceAttr(credentials, "tls_server_name", "db-1.stackshift.internal"),
				resource.TestCheckResourceAttr(credentials, "endpoints.#", "2"),
				resource.TestCheckResourceAttr(credentials, "endpoints.1.role", "read"),
				resource.TestCheckResourceAttr(credentials, "endpoints.1.connection_string", "postgresql://read"),
				resource.TestCheckResourceAttr(credentials, "endpoints.1.ca_filename", "ca.pem"),
			),
		}},
	})
}

func TestDedicatedDatabaseConfigValidation(t *testing.T) {
	startFakeDedicatedAPI(t, &fakeDedicatedAPI{})
	config := func(name, window, extra string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database" "orders" {
  name           = %q
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20
%s
  maintenance_window = {
    %s
  }
}
`, name, extra, window)
	}
	validWindow := "day_of_week = 1\n    start_minute = 60"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{Config: config("orders", "day_of_week = 7\n    start_minute = 0", ""), ExpectError: regexp.MustCompile(`day_of_week\s+is\s+0\s+\(Sunday\)\s+to\s+6`)},
			{Config: config("orders", "day_of_week = 1\n    start_minute = 1440", ""), ExpectError: regexp.MustCompile(`start_minute\s+is\s+minutes\s+after\s+midnight\s+UTC,\s+0\s+to\s+1439`)},
			{
				Config:      config("orders", "day_of_week = 9\n    start_minute = 2000", ""),
				ExpectError: regexp.MustCompile(`(?s)day_of_week\s+is\s+0.*start_minute\s+is\s+minutes`),
			},
			{Config: config("orders", "day_of_week = null\n    start_minute = 5", ""), ExpectError: regexp.MustCompile(`day_of_week\s+is\s+required`)},
			{Config: config("orders", "day_of_week = 2\n    start_minute = null", ""), ExpectError: regexp.MustCompile(`start_minute\s+is\s+required`)},
			{Config: config(" orders", validWindow, ""), ExpectError: regexp.MustCompile(`Remove\s+the\s+surrounding\s+spaces`)},
			{Config: config("orders", validWindow, `  engine = "postgre"`), ExpectError: regexp.MustCompile(`Use\s+postgres\s+or\s+mysql`)},
			{Config: config("orders", validWindow, `  version = "seventeen"`), ExpectError: regexp.MustCompile(`Use\s+a\s+version\s+number`)},
			{Config: config("orders", validWindow, `  version = "17.4.1"`), ExpectError: regexp.MustCompile(`Use\s+a\s+version\s+number`)},
		},
	})
}

func TestDedicatedDatabaseEngineAndVersionChangesNeedAllowReplace(t *testing.T) {
	fastPolling(t)
	fake := &fakeDedicatedAPI{}
	startFakeDedicatedAPI(t, fake)
	name := "stackshift_dedicated_database.orders"
	config := func(engine, version, extra string) string {
		return fmt.Sprintf(`
resource "stackshift_dedicated_database" "orders" {
  name           = "orders"
  engine         = %q
  version        = %q
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20
%s
}
`, engine, version, extra)
	}
	refusal := regexp.MustCompile(`(?s)replaces\s+the\s+database\s+and\s+deletes\s+its\s+data.*allow_replace\s+=\s+true`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		Steps: []resource.TestStep{
			{Config: config("postgres", "17", ""), Check: resource.TestCheckResourceAttr(name, "allow_replace", "false")},
			{Config: config("postgres", "16", ""), ExpectError: refusal},
			{Config: config("mysql", "8.4", ""), ExpectError: refusal},
			{
				Config:           config("postgres", "16", "  allow_replace = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate)}},
				Check: func(*terraform.State) error {
					if fake.createCount != 2 || fake.createBody["version"] != "16" {
						return fmt.Errorf("creates = %d, body = %v", fake.createCount, fake.createBody)
					}
					return nil
				},
			},
			{ResourceName: name, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"master_password", "allow_replace"}},
		},
	})
}

func dedicatedModelFixture() dedicatedDatabaseModel {
	return dedicatedDatabaseModel{
		ID: types.StringValue("db-1"), ProjectID: types.StringNull(), Name: types.StringValue("orders"), Engine: types.StringValue("postgres"), Version: types.StringValue("17"),
		InstanceClass: types.StringValue("s-2vcpu-4gb"), StorageGB: types.Int64Value(20), StorageMaxGB: types.Int64Value(40), AllocatedStorageGB: types.Int64Value(20),
		MasterUsername: types.StringNull(), DatabaseName: types.StringValue("app"), BackupRetentionDays: types.Int64Value(7), DeletionProtection: types.BoolValue(false),
		MaintenanceWindow: types.ObjectValueMust(maintenanceWindowType.AttrTypes, map[string]attr.Value{"day_of_week": types.Int64Value(1), "start_minute": types.Int64Value(60)}),
		Billing:           types.StringValue("monthly"), Standby: types.StringValue("off"), OnStandbyLoss: types.StringValue("degrade"), ApplyChanges: types.StringValue("window"),
		AcceptPriceChanges: types.BoolValue(false), SkipFinalSnapshot: types.BoolValue(false), MasterPassword: types.StringNull(), Status: types.StringValue("running"),
		Host: types.StringNull(), Port: types.Int64Null(), Endpoints: types.ListNull(dedicatedEndpointType), BillingStatus: types.StringValue("active"), CheckoutURL: types.StringNull(),
		RestoreFrom: types.ObjectNull(restoreFromAttributeTypes), AllowReplace: types.BoolValue(false),
		Timeouts: timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{"create": types.StringType, "update": types.StringType, "delete": types.StringType})},
	}
}

func modifyDedicatedPlan(t *testing.T, state, plan dedicatedDatabaseModel, config dedicatedDatabaseModel) frameworkresource.ModifyPlanResponse {
	ctx := context.Background()
	r := &dedicatedDatabaseResource{}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx)
	raw := func(model dedicatedDatabaseModel) tftypes.Value {
		built := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
		if diags := built.Set(ctx, &model); diags.HasError() {
			t.Fatal(diags)
		}
		return built.Raw
	}
	req := frameworkresource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: raw(config)},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: raw(plan)},
		State:  tfsdk.State{Schema: schemaResp.Schema, Raw: raw(state)},
	}
	resp := frameworkresource.ModifyPlanResponse{Plan: req.Plan}
	r.ModifyPlan(ctx, req, &resp)
	return resp
}

func TestDedicatedDatabaseModifyPlanGuardsReplacementAndWarnsAboutRestoreFrom(t *testing.T) {
	state := dedicatedModelFixture()

	engineChange := dedicatedModelFixture()
	engineChange.Engine, engineChange.Version = types.StringValue("mysql"), types.StringValue("8.4")
	refused := modifyDedicatedPlan(t, state, engineChange, engineChange)
	if len(refused.Diagnostics.Errors()) != 2 || !strings.Contains(refused.Diagnostics.Errors()[0].Detail(), "deletes its data") {
		t.Fatalf("diagnostics = %v", refused.Diagnostics)
	}

	engineChange.AllowReplace = types.BoolValue(true)
	if allowed := modifyDedicatedPlan(t, state, engineChange, engineChange); allowed.Diagnostics.HasError() {
		t.Fatalf("diagnostics = %v", allowed.Diagnostics)
	}

	unknownVersion := dedicatedModelFixture()
	unknownVersion.Version = types.StringUnknown()
	if pending := modifyDedicatedPlan(t, state, unknownVersion, unknownVersion); len(pending.Diagnostics.Errors()) != 1 {
		t.Fatalf("a version known only at apply may replace the database, diagnostics = %v", pending.Diagnostics)
	}

	restore := dedicatedModelFixture()
	restore.RestoreFrom = types.ObjectValueMust(restoreFromAttributeTypes, map[string]attr.Value{
		"source_database_id": types.StringValue("src-1"), "target_time": types.StringValue("2026-10-25T14:30:00Z"),
		"backup_id": types.StringNull(), "from_mirror": types.BoolNull(),
	})
	warned := modifyDedicatedPlan(t, state, restore, restore)
	if warned.Diagnostics.HasError() || len(warned.Diagnostics.Warnings()) != 1 || !strings.Contains(warned.Diagnostics.Warnings()[0].Detail(), "only matters when a database is created") {
		t.Fatalf("diagnostics = %v", warned.Diagnostics)
	}

	if quiet := modifyDedicatedPlan(t, state, state, state); len(quiet.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", quiet.Diagnostics)
	}
}

func TestDedicatedDatabaseRemovedFromStateWhenDeleting(t *testing.T) {
	fastPolling(t)
	fake := &fakeDedicatedAPI{}
	startFakeDedicatedAPI(t, fake)
	config := dedicatedDatabaseConfig("s-2vcpu-4gb", "  timeouts = {\n    create = \"30m\"\n    delete = \"30m\"\n  }")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: dedicatedFactories,
		CheckDestroy: func(*terraform.State) error {
			if !fake.deleted || fake.deleteReads != 2 {
				return fmt.Errorf("deleted = %v after %d reads", fake.deleted, fake.deleteReads)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttr("stackshift_dedicated_database.orders", "id", "db-1")},
			{
				PreConfig: func() { fake.mu.Lock(); fake.status = "deleting"; fake.mu.Unlock() },
				Config:    config,
				Check: func(*terraform.State) error {
					if fake.createCount != 2 {
						return fmt.Errorf("creates = %d", fake.createCount)
					}
					return nil
				},
			},
		},
	})
}

func TestPollDedicatedRetriesServerAndTransportErrors(t *testing.T) {
	fastPolling(t)
	calls := 0
	err := pollDedicated(context.Background(), time.Minute, func(context.Context) (bool, error) {
		calls++
		switch calls {
		case 1:
			return false, &client.DedicatedError{StatusCode: http.StatusServiceUnavailable}
		case 2:
			return false, &url.Error{Op: "Get", URL: "https://api.example.test", Err: io.EOF}
		case 3:
			return false, nil
		}
		return true, nil
	})
	if err != nil || calls != 4 {
		t.Fatalf("err = %v after %d calls", err, calls)
	}
}

func TestPollDedicatedFailsAtOnceOnClientErrors(t *testing.T) {
	fastPolling(t)
	for name, failure := range map[string]error{
		"bad request": &client.DedicatedError{StatusCode: http.StatusBadRequest},
		"not found":   fmt.Errorf("%w: gone", client.ErrNotFound),
		"conflict":    &client.DedicatedError{StatusCode: http.StatusConflict},
		"other":       fmt.Errorf("the snapshot failed"),
	} {
		calls := 0
		err := pollDedicated(context.Background(), time.Minute, func(context.Context) (bool, error) {
			calls++
			return false, failure
		})
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("%s: err = %v after %d calls", name, err, calls)
		}
	}
}

func TestPollDedicatedReportsLastErrorAtDeadline(t *testing.T) {
	fastPolling(t)
	err := pollDedicated(context.Background(), 20*time.Millisecond, func(context.Context) (bool, error) {
		return false, &client.DedicatedError{StatusCode: http.StatusBadGateway, Message: "upstream down"}
	})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "upstream down") {
		t.Fatalf("err = %v", err)
	}
}

func TestPollDedicatedRetriesRateLimitAndTimeoutResponses(t *testing.T) {
	fastPolling(t)
	calls := 0
	err := pollDedicated(context.Background(), time.Minute, func(context.Context) (bool, error) {
		calls++
		switch calls {
		case 1:
			return false, &client.DedicatedError{StatusCode: http.StatusTooManyRequests}
		case 2:
			return false, &client.DedicatedError{StatusCode: http.StatusRequestTimeout}
		}
		return true, nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err = %v after %d calls", err, calls)
	}
}

func TestPollDedicatedHonoursRetryAfterWithinTheTimeout(t *testing.T) {
	fastPolling(t)
	var first, second time.Time
	calls := 0
	err := pollDedicated(context.Background(), time.Minute, func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			first = time.Now()
			return false, &client.DedicatedError{StatusCode: http.StatusTooManyRequests, RetryAfter: 60 * time.Millisecond}
		}
		second = time.Now()
		return true, nil
	})
	if err != nil || second.Sub(first) < 55*time.Millisecond {
		t.Fatalf("err = %v, waited %v", err, second.Sub(first))
	}

	started := time.Now()
	err = pollDedicated(context.Background(), 40*time.Millisecond, func(context.Context) (bool, error) {
		return false, &client.DedicatedError{StatusCode: http.StatusTooManyRequests, RetryAfter: time.Hour}
	})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 2*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(started))
	}
}

func TestPollDedicatedFailsAtOnceOnCertificateErrors(t *testing.T) {
	fastPolling(t)
	for name, failure := range map[string]error{
		"unknown authority":   &url.Error{Op: "Get", URL: "https://api.example.test", Err: x509.UnknownAuthorityError{}},
		"invalid certificate": &url.Error{Op: "Get", URL: "https://api.example.test", Err: x509.CertificateInvalidError{Reason: x509.Expired}},
		"wrong hostname":      &url.Error{Op: "Get", URL: "https://api.example.test", Err: x509.HostnameError{Host: "api.example.test"}},
		"record header":       &url.Error{Op: "Get", URL: "https://api.example.test", Err: tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}},
		"verification":        &url.Error{Op: "Get", URL: "https://api.example.test", Err: &tls.CertificateVerificationError{Err: errors.New("certificate has expired")}},
	} {
		calls := 0
		err := pollDedicated(context.Background(), time.Minute, func(context.Context) (bool, error) {
			calls++
			return false, failure
		})
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("%s: err = %v after %d calls", name, err, calls)
		}
	}
}
