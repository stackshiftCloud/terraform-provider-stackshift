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

All outputs are marked sensitive except `tls_server_name` and `ca_certificate`:

- `username`
- `password`
- `database_name`
- `host`
- `port`
- `tls_mode`
- `connection_string`
- `endpoints`, every address the database answers on (including read replicas of a Dedicated database), each with `protocol`, `network`, `role`, `primary`, `host`, `port`, `tls_mode`, `tls_server_name`, `connection_string` and `ca_filename`

`tls_server_name` and `ca_certificate` are the name to check and the certificate authority to trust when you connect with certificate verification.

Use this data source to wire a managed database into project environment variables:

```hcl
resource "stackshift_project_env" "production" {
  project_id = stackshift_project.api.id

  variables = {
    DATABASE_URL = data.stackshift_database_credentials.postgres.connection_string
  }
}
```


## `stackshift_regions`

Lists the regions where StackShift can run each product right now, from `GET /regions`. Set `product` to `apps`, `databases` or `vps` to keep only regions where that product is available.

```hcl
data "stackshift_regions" "apps" {
  product = "apps"
}

resource "stackshift_project" "api" {
  name   = "example-api"
  region = data.stackshift_regions.apps.default_region
}
```

Computed fields: `default_region` and `regions`, a list with `id`, `name`, `city`, `country`, `country_code`, `default`, `status` (`available` or `degraded`), `apps_available`, `app_tiers` (`shared`, `isolated`), `databases_available`, `database_tiers` (`starter`) and `vps_available`.

## `stackshift_app_sizes`

Lists the app sizes and each plan's limits. `stackshift_project` has no size attribute yet, so set the size with the CLI or API `resource_preset`, from `GET /app-sizes`. Set `listed_only = true` to keep only the sizes shown in the dashboard's size picker.

```hcl
data "stackshift_app_sizes" "picker" {
  listed_only = true
}

output "app_size_slugs" {
  value = [for size in data.stackshift_app_sizes.picker.sizes : size.slug]
}
```

Computed fields: `version`, `currency`, `sizes` (`slug`, `name`, `description`, `memory_mb`, `cpu_millicores`, `listed`, `recommended`, `price_ngn_monthly`, `included_in_plan`, `min_plan`, `plans`) and `plans` (`slug`, `name`, `max_memory_mb`, `max_cpu_millicores`, `app_tier`, `default_size`). `price_ngn_monthly` is null when the size is included in the plan, and `max_cpu_millicores = -1` means unlimited.

## `stackshift_database_instance_classes`

Lists the sizes you can pick for a Dedicated database, from `GET /databases/instance-classes`. Dedicated databases must be switched on for your account; otherwise the API answers 403. See current prices at https://stackshift.cloud/pricing.

```hcl
data "stackshift_database_instance_classes" "all" {}

output "dedicated_cpu_classes" {
  value = [for class in data.stackshift_database_instance_classes.all.classes : class.slug if class.cpu_policy == "dedicated"]
}
```

Computed field: `classes`, a list with `slug`, `vcpu`, `memory_mb`, `cpu_policy` (`shared` or `dedicated`), `max_connections`, `pooled_max_connections`, `disk_iops`, `disk_bytes_per_sec`, `network_kbps`, `active`, `currency`, `monthly_price_minor`, `storage_gb_month_price_minor` and `backup_gb_month_price_minor`. Price fields are passed through from the API in the currency's smallest unit.

## `stackshift_dedicated_database`

Reads a Dedicated database by id, from `GET /databases/{id}`.

```hcl
data "stackshift_dedicated_database" "orders" {
  id = "database-uuid"
}

output "orders_endpoints" {
  value = data.stackshift_dedicated_database.orders.endpoints
}
```

Computed fields: `project_id`, `name`, `engine` (`postgres` or `mysql`), `version`, `status`, `instance_class`, `allocated_storage_gb`, `storage_max_gb`, `backup_retention_days`, `deletion_protection`, `maintenance_window` (`day_of_week`, `start_minute`), `billing`, `billing_status`, `standby`, `on_standby_loss`, `master_username`, `database_name`, `host`, `port`, `tls_mode`, `endpoints` (`protocol`, `network`, `role`, `host`, `port`, `tls_mode`, `primary`) and `members`, a list with `id`, `role` (`primary`, `standby` or `replica`), `instance_class`, `status` and `site`. Passwords are never returned; use `stackshift_database_credentials` for connection details.

## `stackshift_dedicated_database_backups`

Lists the backups of a Dedicated database and the time window it can be restored to, from `GET /databases/{id}/backups`. Use it to pick a `target_time` or a `backup_id` for `restore_from`. The target you choose has to lie inside the restorable window, and it should be written out explicitly, as a literal time or a snapshot id, rather than pointing at `restorable_until`, which moves with every backup.

```hcl
variable "source_database_id" {
  description = "An existing Dedicated database that already has backups."
  type        = string
}

data "stackshift_dedicated_database_backups" "source" {
  database_id = var.source_database_id
}

resource "stackshift_dedicated_database" "restored" {
  name           = "orders-restored"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20

  restore_from = {
    source_database_id = var.source_database_id
    target_time        = "2026-10-25T14:30:00Z"
  }
}
```

Computed fields: `retention_days`, `restorable_from` and `restorable_until` (RFC 3339 times, empty until a first backup exists) and `backups`, a list with `id`, `kind` (`scheduled`, `manual` or `final`), `repo`, `name`, `status`, `size_bytes`, `started_at` and `finished_at`.

## `stackshift_database_parameter_definitions`

Lists the engine settings you can put in a parameter group for one engine version, from `GET /databases/parameter-definitions`.

```hcl
data "stackshift_database_parameter_definitions" "postgres17" {
  family = "postgres17"
}

output "settings_that_need_a_restart" {
  value = [for parameter in data.stackshift_database_parameter_definitions.postgres17.parameters : parameter.name if parameter.needs_restart]
}
```

Argument: `family`, the engine and version, such as `postgres17` or `mysql8.4`. StackShift rejects families it doesn't offer. Computed field: `parameters`, a list with `name`, `kind`, `min`, `max`, `values` (for settings that take a fixed set), `needs_restart`, `engine_default` and `description`.
