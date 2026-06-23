resource "stackshift_database" "postgres" {
  project_id = stackshift_project.api.id
  name       = "example-postgres"
  type       = "postgres"
  version    = "16"
  size_gb    = 10
}

data "stackshift_database_credentials" "postgres" {
  database_id = stackshift_database.postgres.id
}
