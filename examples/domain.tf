resource "stackshift_domain" "api" {
  project_id       = stackshift_project.api.id
  domain           = "api.example.com"
  verify_on_create = true
}
