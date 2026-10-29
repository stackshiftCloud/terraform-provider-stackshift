resource "stackshift_database" "postgres" {
  project_id          = stackshift_project.api.id
  name                = "example-postgres"
  type                = "postgres"
  version             = "16"
  size_gb             = 10
  sleep_after_minutes = 15
}

resource "stackshift_database_branch" "pr_142" {
  database_id         = stackshift_database.postgres.id
  name                = "pr-142"
  ttl                 = "7d"
  sleep_after_minutes = 5
  source_ref          = "pr:142"
}

resource "stackshift_database_branch" "before_incident" {
  database_id = stackshift_database.postgres.id
  name        = "before-incident"

  from = {
    time = "2026-10-25T14:30:00Z"
  }
}

data "stackshift_database_credentials" "pr_142" {
  database_id = stackshift_database_branch.pr_142.id
}

output "pr_142_connection_string" {
  value     = data.stackshift_database_credentials.pr_142.connection_string
  sensitive = true
}
