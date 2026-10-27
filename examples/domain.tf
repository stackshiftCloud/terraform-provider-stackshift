resource "stackshift_domain" "api" {
  project_id       = stackshift_project.api.id
  domain           = "api.example.com"
  verify_on_create = true
}

resource "stackshift_domain" "root" {
  project_id = stackshift_project.api.id
  domain     = "example.com"
  primary    = true
}

resource "stackshift_domain" "www" {
  project_id = stackshift_project.api.id
  domain     = "www.example.com"
  depends_on = [stackshift_domain.root]
}
