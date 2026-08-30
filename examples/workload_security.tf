resource "stackshift_workload_external_dependency" "neon" {
  project_id   = stackshift_project.api.id
  environment  = "production"
  hostname     = "ep-example.eu-central-1.aws.neon.tech"
  port         = 5432
  protocol     = "tls"
  purpose      = "Primary PostgreSQL database"
  tls_required = true
}

resource "stackshift_workload_egress_grant" "neon" {
  project_id    = stackshift_project.api.id
  dependency_id = stackshift_workload_external_dependency.neon.id
  environment   = "production"
  protocol      = "tls"
  hostname      = stackshift_workload_external_dependency.neon.hostname
  port_start    = 5432
  port_end      = 5432
  purpose       = "Primary PostgreSQL database"
  expires_at    = "2026-11-18T00:00:00Z"
}
