# Data Sources

## `stackshift_project`

Looks up a StackShift project by `id` or by `name`.

```hcl
data "stackshift_project" "api" {
  name = "example-api"
}
```

Computed fields include `id`, `runtime`, `region`, `port`, `status`, `deployment_mode`, and `created_at`.

## `stackshift_database`

Looks up a StackShift database by `id`.

```hcl
data "stackshift_database" "postgres" {
  id = "database-uuid"
}
```

Computed fields include `project_id`, `name`, `type`, `version`, `size_gb`, `status`, `host`, `port`, `tls_mode`, `database_name`, and `created_at`.

## `stackshift_database_credentials`

Reads database connection credentials through `/databases/{databaseID}/credentials`.

```hcl
data "stackshift_database_credentials" "postgres" {
  database_id = stackshift_database.postgres.id
}
```

All outputs are marked sensitive:

- `username`
- `password`
- `database_name`
- `host`
- `port`
- `tls_mode`
- `connection_string`

Use this data source to wire a managed database into project environment variables:

```hcl
resource "stackshift_project_env" "production" {
  project_id = stackshift_project.api.id

  variables = {
    DATABASE_URL = data.stackshift_database_credentials.postgres.connection_string
  }
}
```

