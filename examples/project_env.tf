resource "stackshift_project_env" "production" {
  project_id  = stackshift_project.api.id
  environment = "production"

  variables = {
    NODE_ENV = "production"
    API_KEY  = var.api_key
  }
}
