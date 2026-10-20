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
