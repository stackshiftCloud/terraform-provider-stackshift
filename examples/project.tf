resource "stackshift_project" "api" {
  name    = "example-api"
  runtime = "node"
  region  = "us-east"
  port    = 3000

  github_repo_url        = "https://github.com/example/example-api"
  github_repo_id         = 123456789
  github_installation_id = 987654321
  github_branch          = "main"

  build_command = "npm run build"
  start_command = "npm start"
}
