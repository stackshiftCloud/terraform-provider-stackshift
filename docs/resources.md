# Resources

## `stackshift_workload_external_dependency`

Declares one exact external service used by a hosted project. Declaration records the hostname, port, protocol, purpose, and TLS requirement, but does not approve network access.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Owning project; replaces the resource if changed. |
| `environment` | no | Defaults to `production`; replaces the resource if changed. |
| `hostname` | yes | Exact service hostname. Do not put credentials or a complete connection URL here. |
| `port` | yes | Exact destination port. |
| `protocol` | no | Defaults to `tls`. |
| `purpose` | yes | Human-readable reason the workload needs the destination. |
| `tls_required` | no | Defaults to `true`. |

`private_route_required`, `detected`, and `status` are computed. Destroying this resource disables the declaration and revokes its active grants while retaining the security audit record.

## `stackshift_workload_egress_grant`

Requests scoped, expiring outbound access for a hosted workload. StackShift operator approval remains a separate action and cannot be performed by Terraform.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Owning project; replaces the resource if changed. |
| `dependency_id` | no | ID of the related `stackshift_workload_external_dependency`. |
| `environment` | no | Defaults to `production`. |
| `protocol` | no | Defaults to `tls`. |
| `hostname` | no | Exact approved hostname. Use either an exact hostname or CIDR. |
| `cidr` | no | Exact approved CIDR where hostname policy is not appropriate. |
| `port_start` | yes | First allowed destination port. |
| `port_end` | yes | Last allowed destination port. |
| `purpose` | yes | Human-readable operational purpose. |
| `broad_public_egress` | no | Defaults to `false`; broad requests require explicit operator approval and expire within seven days. |
| `expires_at` | yes | RFC3339 expiry. Destination-scoped grants may last up to 90 days. |
| `idempotency_key` | no | Stable create key; deterministically derived from the immutable request when omitted. |

`status` is computed and remains pending until an operator records a decision. Destroying this resource revokes the request while retaining its security audit record.

## `stackshift_project`

Manages a StackShift project through `/api/v1/projects`.

Normal StackShift-managed deployments do not require a node ID. Leave placement to StackShift unless a separate resource explicitly exposes placement.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Project display name. |
| `runtime` | yes | Runtime identifier accepted by StackShift. |
| `region` | no | Region id from `GET /api/v1/regions` (or `stackshift project regions`), for example `hel1`. Leave it out to let StackShift choose. An app cannot move to another region after it is created. |
| `port` | no | Application port. |
| `description` | no | Project description. |
| `team_id` | no | Team/workspace owner. |
| `github_repo_url` | no | GitHub repository URL. |
| `github_repo_id` | no | GitHub repo ID. |
| `github_installation_id` | no | GitHub App installation ID. |
| `github_branch` | no | Branch to deploy. |
| `build_command` | no | Build command. |
| `start_command` | no | Start command. |
| `install_command` | no | Install command. |
| `root_directory` | no | Repository root directory. |
| `output_directory` | no | Build output directory. |
| `source_type` | no | Source type, for example GitHub or Docker image. |
| `docker_image_uri` | no | Container image URI. |

### Import

```sh
terraform import stackshift_project.api <project_id>
```

## `stackshift_project_env`

Owns the complete environment-variable set for one project environment.

Values managed outside Terraform will be overwritten on apply.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Replaces the resource if changed. |
| `environment` | no | Defaults to `production`; replaces the resource if changed. |
| `variables` | yes | Sensitive map of string values. |

### Import

```sh
terraform import stackshift_project_env.production <project_id>:production
```

## `stackshift_database`

Manages a StackShift managed database attached to a project.

Shape fields such as `project_id`, `type`, `version`, `size_gb`, and `target_node_id` replace the resource when changed.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Owning project ID. |
| `name` | yes | Database name. |
| `type` | yes | `postgres`, `mysql`, `mariadb`, `clickhouse`, `mongodb`, `redis` (Redis 8), or `valkey` (Valkey 9, Redis-compatible). |
| `version` | yes | Version accepted by StackShift. For `redis` use `"8.10"`, for `valkey` use `"9.0"`; ClickHouse accepts `"26.8"` or `"26.3"`; for `mongodb` use `"8.0"`. |
| `size_gb` | yes | Managed database disk size. |
| `target_node_id` | no | Optional explicit node placement target. |
| `sleep_after_minutes` | no | Sleep when idle, for `postgres` only: the database sleeps after this many minutes without use, 5 to 1440, and wakes on the next connection. Leave it out or set `0` to keep it awake. Changes in place. It is turned on right after the database is created; if StackShift can't do that yet, the apply finishes with a warning and the next apply tries again. |

Read the sleep setting back with `sleep_after_minutes`; if it is changed outside Terraform, the next plan shows the difference.

### Import

```sh
terraform import stackshift_database.postgres <database_id>
```

## `stackshift_database_branch`

Manages a Branch of a PostgreSQL database through `/api/v1/databases/{database_id}/branches`: a separate working copy that you can change, test against and delete without touching the original. A branch is a database of its own, so it has its own connection details and its own `id`. It counts against your plan's limits on branches and on branches that are awake at once.

Creating a branch waits until the branch is ready, and deleting it waits until it is gone. If StackShift can't finish the branch, the apply fails with the reason, the failed branch is deleted, and nothing is kept in state.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The database to branch from: a main database or another branch. **Changing it replaces the branch.** |
| `name` | yes | The branch's name. Changes in place. |
| `from` | no | Where the branch's data starts from. Set exactly one of `now = true`, `time` (an RFC 3339 time inside the database's history window), `restore_point_id` or `backup_id`. Defaults to `now`. **Changing it replaces the branch.** |
| `ttl` | no | How long the branch lives before StackShift deletes it, such as `24h` or `7d`, or `never`. Leave it out for the plan's default. Changes in place; removing it after it was set changes the branch to `never`. |
| `sleep_after_minutes` | no | Sleep when idle for the branch, 5 to 1440 minutes. Leave it out or set `0` to keep it awake. Changes in place. |
| `source_ref` | no | A label that says what the branch is for, such as `pr:142`. **Changing it replaces the branch.** |
| `instance_class` | no | For a branch of a Dedicated database: the size the branch is built and priced at, a slug from `stackshift_database_instance_classes`. Leave it out to use the parent's size. Branches of Starter databases have no size of their own and it stays empty. **Changing it replaces the branch.** |
| `accept_price_changes` | no | Defaults to `false`. A branch of a Dedicated database costs extra; see paid branches below. Only used when the branch is created. Import sets it to `false`. |
| `timeouts` | no | `create` and `delete` durations, each 45 minutes by default. |

Computed attributes: `id` (the branch's own database ID), `instance_class` (when left out), `root_database_id` (the main database it descends from), `mechanism` (`instant`, `copy`, `point_in_time` or `backup`), `state` (`creating`, `ready`, `resetting`, `promoting`, `failed`, `deleting` or `needs_recovery`), `private_hostname`, `branched_at` and `expires_at` (empty when the branch never expires).

Making a branch the main database and resetting a branch are not Terraform actions. Do them from the dashboard or the API.

### Paid branches

Branches of Starter databases are never charged. A branch of a Dedicated database is: StackShift answers the first create with a quote. With `accept_price_changes = false` the apply stops and shows the charge, the new monthly price and the currency. Set `accept_price_changes = true` and apply again; the provider then confirms the quoted charge, takes it from the wallet and adds a warning with the amount. If the wallet doesn't cover it, or the parent database's plan is unpaid, the apply stops and says so. Prices: https://stackshift.cloud/pricing

### Connecting to a branch

Read the branch's connection details with `stackshift_database_credentials`, using the branch's `id`:

```hcl
resource "stackshift_database_branch" "pr_142" {
  database_id = stackshift_database.postgres.id
  name        = "pr-142"
  ttl         = "7d"
  source_ref  = "pr:142"
}

data "stackshift_database_credentials" "pr_142" {
  database_id = stackshift_database_branch.pr_142.id
}
```

### Import

```sh
terraform import stackshift_database_branch.pr_142 <parent_database_id>/<branch_id>
```

Import reads the name, label, sleep setting and starting point from StackShift. It can't recover `ttl`, so the first plan after an import shows `ttl` as a change if you set it in your configuration.

## `stackshift_domain`

Connects a custom hostname to a project. The hostname serves the project once it is verified. Verification needs proof that the hostname belongs to this project: a CNAME to the project address (shown by `stackshift project-domain dns-config PROJECT`), or the TXT record in `verification_txt_name` and `verification_txt_value`. An A record alone is not enough because every project on a server shares that address. Root domains need the TXT record. A hostname that stays unverified for 7 days can be claimed by another project.

Every hostname is its own resource. Connecting `example.com` does not serve `www.example.com`; add a second `stackshift_domain` for `www.example.com`. It verifies through the root domain's proof once the root domain is verified on the same project. A hostname can belong to only one project, and one used by a stack can't be connected to a project.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Replaces the resource if changed. |
| `domain` | yes | Hostname to connect. Replaces the resource if changed. |
| `primary` | no | Makes this the project's primary domain. Changing it updates the domain in place. A domain becomes primary only once verified; until then the next apply after verification makes it primary. If another domain becomes primary outside Terraform, the next apply makes this one primary again. |
| `verify_on_create` | no | Checks DNS again after creation if the first check did not pass. |

### Attributes

| Name | Notes |
| --- | --- |
| `is_primary` | Whether the domain is the project's primary domain right now. |
| `verification_status` | `pending`, `verified` or `failed`. |
| `verified` | `true` once the hostname is proven to belong to the project. |
| `verification_txt_name` | TXT record name that proves ownership. Empty once verified. |
| `verification_txt_value` | TXT record value that proves ownership. Empty once verified. |
| `ssl_status` | Certificate status for the hostname. |
| `dns_warning` | Set when the hostname's DNS points at another project's address, with the record to fix. Empty otherwise. |

### Import

```sh
terraform import stackshift_domain.api <project_id>:<domain_id>
```

## `stackshift_dns_record`

Manages a DNS record for a registered StackShift-managed domain.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `domain_id` | yes | Registered domain ID; replaces the resource if changed. |
| `type` | yes | `A`, `AAAA`, `CNAME`, `TXT`, `MX`, `SPF`, or `SRV`. |
| `name` | yes | Record name. |
| `value` | yes | Record value. |
| `ttl` | no | Defaults to `3600`. |
| `priority` | no | Used for record types such as `MX` and `SRV`. |

### Import

```sh
terraform import stackshift_dns_record.www <domain_id>:<record_id>
```

## `stackshift_build_action`

Triggers a project build when the resource is created.

Action resources are not reversible desired state. Deleting the Terraform resource only removes Terraform state.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Project to build. |
| `branch` | no | Branch to build. |
| `commit_sha` | no | Commit SHA to build. |
| `reason` | no | Human-readable reason. |
| `nonce` | no | Optional uniqueness key for repeated actions. |

## `stackshift_deployment_action`

Triggers a deployment operation such as redeploy or rollback.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Target project. |
| `action` | yes | Deployment action accepted by the API. |
| `deployment_id` | no | Existing deployment target where applicable. |
| `reason` | no | Human-readable reason. |
| `nonce` | no | Optional uniqueness key for repeated actions. |

## `stackshift_compute_instance`

Provisions a VM/VPS-style compute instance. This is separate from managed app project placement.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Instance display name. |
| `hostname` | yes | Hostname to assign. |
| `image_slug` | yes | Image slug accepted by StackShift. |
| `ssh_public_key` | yes | Sensitive SSH public key material. |
| `mode` | yes | Provisioning mode accepted by StackShift. |
| `plan_id` | no | Explicit plan ID. |
| `plan_slug` | no | Plan slug when ID is not used. |
| `install_docker` | no | Defaults to `true`. |
| `install_stackshift_agent` | no | Defaults to `false`. |
| `idempotency_key` | no | Optional idempotency key. |
| `payment_provider` | no | Optional payment provider. |

## `stackshift_compute_action`

Triggers a compute operation such as start, stop, restart, rebuild, or destroy.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `instance_id` | yes | Compute instance ID. |
| `action` | yes | Compute action accepted by StackShift. |
| `confirmation` | no | Confirmation text for destructive actions. |
| `image_slug` | no | Rebuild image slug where applicable. |
| `ssh_public_key` | no | Sensitive replacement SSH key where applicable. |
| `hostname` | no | Replacement hostname where applicable. |
| `nonce` | no | Optional uniqueness key for repeated actions. |

## `stackshift_agency_client`

Manages an agency or white-label client record.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Client display name. |
| `slug` | no | Stable client slug. |
| `email` | no | Client contact email. |
| `company` | no | Company name. |
| `notes` | no | Internal notes. |

## `stackshift_agency_resource_assignment`

Assigns a StackShift resource to an agency client.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `client_id` | yes | Agency client ID. |
| `resource_type` | yes | Resource type being assigned. |
| `resource_id` | yes | StackShift resource ID. |
| `role` | no | Assignment role. |
| `notes` | no | Internal notes. |

## `stackshift_runbook`

Manages an operational runbook definition.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Runbook name. |
| `description` | no | Runbook description. |
| `steps` | yes | JSON or serialized runbook steps. |
| `version` | no | Runbook version label. |
| `enabled` | no | Defaults to `true`. |

## `stackshift_runbook_execution`

Triggers a runbook execution when the resource is created.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `runbook_id` | yes | Runbook to execute. |
| `project_id` | no | Target project. |
| `environment` | no | Target environment. |
| `parameters` | no | JSON or serialized execution parameters. |
| `reason` | no | Human-readable reason. |
| `nonce` | no | Optional uniqueness key for repeated executions. |

## `stackshift_asset_bucket`

Manages an Assets bucket, including visibility, upload policy, CORS, replication, lifecycle JSON, and an optional custom domain association.

## `stackshift_mail_domain`

Manages a StackShift Mail sending domain. Set exactly one of `domain` or `registered_domain_id`. `verify_on_create` performs a DNS verification attempt after creation. Import with the Mail domain UUID.

## `stackshift_mail_webhook`

Manages a signed StackShift Mail event webhook. `event_types` uses current recipient events such as `mail.message.mta_accepted`, `mail.delivery.delivered`, `mail.delivery.delayed`, `mail.message.bounced`, and `mail.message.complained`. The HMAC `secret` is returned once, is marked sensitive, and is not recoverable when importing an existing webhook.

## `stackshift_asset_webhook`

Manages a signed Assets event subscription. The `secret` attribute is sensitive and is returned only at creation time; retain the Terraform state securely.

## `stackshift_asset_lifecycle_rule`

Manages an immutable lifecycle rule. `action` is `delete` or `expire_versions`; changing the name, prefix, action, age, or enabled state replaces the rule because the API intentionally exposes create/list/delete semantics.

## `stackshift_asset_domain`

Manages a custom Assets delivery domain. Apply first with `verify = false`, publish `verification_name` and `verification_value` in DNS, then set `verify = true` to invoke ownership verification and TLS provisioning on the next apply.

## `stackshift_byoc_provider_connection`

Manages one clean-contract BYOCloud provider identity and validates it during apply by default.

- `cloud_provider = "hetzner"` or `"digitalocean"` requires `token`. Use a provider-scoped read/write token.
- `cloud_provider = "aws"` requires `role_arn` and `region`. StackShift returns `external_id`; put that exact value in the IAM role trust policy. AWS access keys are not accepted.
- `cloud_provider = "azure"` requires `azure_tenant_id`, `azure_subscription_id`, and `azure_client_id`. Configure the issuer, subject, and audience returned by StackShift on the Azure federated credential. Client secrets are not accepted.
- `missing_permissions` contains the exact failed permission checks when validation fails.

Provider credentials are sensitive but remain Terraform inputs, so protect remote state with encryption and strict access control. Imported connections can be read without credentials, but credentials must be supplied before changing or revalidating them.

## `stackshift_byoc_node`

Provisions a provider-backed node and waits for the durable provisioning operation to become `active`. The provider resource is discovered by StackShift tags after ambiguous provider responses, so a repeated apply with the same state does not create a duplicate instance. Destroy starts the durable deletion saga and waits until the provider confirms deletion. A `409` deletion blocker is returned to Terraform when projects, stacks, databases, volumes, or dependent snapshots remain.

Required fields are `provider_connection_id`, `cloud_provider`, `region`, `tier_slug`, and `name`. When omitted, `idempotency_key` is deterministically derived from the immutable resource identity and then persisted, so a Terraform process crash cannot turn a retry into a second provider resource. Read-only fields expose operation progress, bootstrap state, enrollment state, overlay address, public address, and failure details.

## `stackshift_byoc_volume`

Creates and attaches a provider-backed volume to a BYOCloud node, then polls the node-scoped resource inventory until it is attached. `node_id`, `name`, `size_gb`, `mount_path`, and `filesystem` describe the immutable attachment. Destroy is refused while dependent snapshots exist.

Import with `NODE_ID:VOLUME_ID`.

## `stackshift_byoc_snapshot`

Creates an inventoried provider snapshot and waits for it to become available. Destroy waits until the snapshot disappears from the node inventory.

Import with `NODE_ID:SNAPSHOT_ID`.

## `stackshift_byoc_static_ip`

Allocates and assigns a stable public IP to a BYOCloud node. Destroy releases the address through the provider-confirmed durable operation. The resource exposes the provider resource ID, address, assignment status, and last synchronization time.

## `stackshift_bucket`

Manages an S3-compatible StackShift S2 bucket through `/api/v1/buckets`.

The API creates one bucket-scoped access key with the bucket. Its secret is returned once and stored in sensitive Terraform state. Protect state with encryption and restricted access. Import can recover the bucket configuration, but it cannot recover an existing secret access key.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Globally unique S3 bucket name; replacing it creates a new bucket. |
| `region` | yes | S2 signing region returned to S3 clients. |
| `visibility` | no | `private` by default; changes replace the bucket. |
| `project_id` | no | Optional owning StackShift project; changes replace the bucket. |
| `access_key_label` | no | Label for the initial bucket-scoped access key. |
| `force_destroy` | no | Defaults to `false`; when true, destroy also removes contained objects. |
| `versioning_enabled` | no | Enables object versioning without replacing the bucket. |
| `quota_bytes` | no | Optional bucket quota; it cannot be lower than current usage. |
| `default_retention_days` | no | Default retention window: `0`, `7`, `30`, or `90` days. |
| `tier_after_days` | no | Tiering threshold: `0`, `7`, `30`, or `90` days. |
| `website_enabled` | no | Enables static website behavior for the bucket. |
| `website_index` | no | Website index object key; defaults to `index.html`. |
| `website_error` | no | Optional website error object key. |
| `encryption_mode` | no | `none`, `sse-s2`, or platform-managed `sse-kms`. |
| `custom_domain_id` | no | Optional active StackShift custom-domain ID. |

Computed attributes include `endpoint`, `access_key_id`, `secret_access_key`, `kms_key_id`, `object_count`, `size_bytes`, `created_at`, and `updated_at`. Customers select `sse-kms`; StackShift selects and returns the platform key. The resource does not accept customer AWS credentials or an arbitrary KMS key ARN.

### Import

```sh
terraform import stackshift_bucket.uploads <bucket_id>
```

## `stackshift_dedicated_database`

Manages a Dedicated PostgreSQL or MySQL database through `/api/v1/databases` (or `/api/v1/projects/{project_id}/databases` when `project_id` is set). Dedicated databases must be switched on for your account; otherwise the API answers 403. Dedicated is a paid product: see https://stackshift.cloud/pricing.

The master password is returned only once, when the database is created. It is stored in sensitive Terraform state as `master_password`, so keep state in an encrypted remote backend with restricted access. Import can't recover it.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | 3 to 40 lowercase letters, digits and dashes. Changes in place; the database keeps its address. |
| `instance_class` | yes | A slug from `stackshift_database_instance_classes`. Changes in place. |
| `storage_gb` | yes | 10 to 1024. Grows in place; it can't shrink. When autogrow raises the size above this value, Terraform doesn't treat it as drift; read the real size from `allocated_storage_gb`. A value below the current size is recorded without a change, since storage can't shrink. |
| `storage_max_gb` | no | Autogrow limit. The server default is twice `storage_gb`, up to 1024. Changes in place. |
| `engine` | no | `postgres` (default) or `mysql`. **Changing it replaces the database and deletes its data.** |
| `version` | no | A version number such as `17` or `8.4`. PostgreSQL `15` to `18` (default `17`) or MySQL `8.4` (default). StackShift rejects versions it doesn't offer when you apply. **Changing it replaces the database and deletes its data.** |
| `allow_replace` | no | Defaults to `false`. Terraform refuses to plan a change to `engine`, `version`, `project_id`, `master_username` or `database_name` (including a value that is only known during apply) unless this is `true`, because the change deletes the database and creates an empty one. To move the data, restore it into a new resource with `restore_from` instead. It doesn't apply to `terraform apply -replace` or `terraform taint`, which you ask for explicitly. Only Terraform uses it; it is never sent to StackShift. Import sets it to `false`. |
| `project_id` | no | Owning project. **Changing it replaces the database and deletes its data.** |
| `master_username` | no | Defaults to `app_owner`. MySQL user names are at most 32 characters. **Changing it replaces the database and deletes its data.** |
| `database_name` | no | Defaults to `app`. **Changing it replaces the database and deletes its data.** |
| `backup_retention_days` | no | 7 to 35; defaults to 7. Changes in place. |
| `deletion_protection` | no | Defaults to `false`. While true, the API refuses to delete the database. Changes in place, so set it to `false` and apply before destroying. |
| `maintenance_window` | no | `{ day_of_week = 0-6, start_minute = 0-1439 }`, in UTC. Picked at random when left out, then read back. Changes in place; a size change waiting for the window moves with it. |
| `billing` | no | `monthly` (default), `yearly` or `hourly`. Changes in place. A switch between prepaid periods takes effect at the next renewal and Terraform shows the new value straight away; a switch away from hourly is charged now and follows the paid-change rules below. |
| `standby` | no | `off` (default), `sync` or `async`. Changes in place. |
| `on_standby_loss` | no | `degrade` (default) or `block`. Changes in place. |
| `apply_changes` | no | When a size change runs: `window` (default, in the maintenance window) or `now`. |
| `accept_price_changes` | no | Defaults to `false`. See paid changes below. |
| `skip_final_snapshot` | no | Defaults to `false`, so destroy takes a final snapshot first. Set `true` to delete without one. |
| `restore_from` | no | Creates the database as a restore of another Dedicated database. Create-only; see restoring below. |
| `sleep_after_minutes` | no | Sleep when idle, for `postgres` only and not with hourly billing: the database sleeps after this many minutes without use, 5 to 1440, and wakes on the next connection. Leave it out or set `0` to keep it awake. Changes in place. It is turned on once the database is running; if StackShift can't do that, the apply finishes with a warning and the next apply tries again. |
| `timeouts` | no | `create`, `update` and `delete` durations. See timeouts below. |

Computed attributes: `id`, `master_password`, `allocated_storage_gb` (the current storage size, including autogrow), `status`, `host`, `port`, `endpoints` (`protocol`, `network`, `role`, `host`, `port`, `tls_mode`, `primary`; read replicas that have caught up appear with their own role), `billing_status` and `checkout_url`.

### MySQL

Set `engine = "mysql"`. MySQL supports the same sizes, storage, standby, read replicas, backups, restore, fast restore points, parameter groups and settings as PostgreSQL. Only version `8.4` is offered, and a major-version upgrade is currently available for PostgreSQL only.

```hcl
resource "stackshift_dedicated_database" "shop" {
  name           = "shop"
  engine         = "mysql"
  instance_class = data.stackshift_database_instance_classes.all.classes[0].slug
  storage_gb     = 20
}
```

### Paying for a new database

When the wallet covers the first payment (or the first hour on hourly billing), the database starts right away. When a card payment is needed, the apply finishes with a warning that contains the checkout link, and the database stays in state with `status = "pending_payment"` and the link in `checkout_url`. It doesn't fail the apply on purpose: a failed create would mark the resource tainted and the next apply would delete it and start over. The account owner pays at the link, then the next `terraform apply` refreshes the database to its running status. Unpaid databases are removed after 24 hours, and the next apply then creates a new one.

### Paid changes

Size changes, standby changes, storage growth and a switch away from hourly billing are prorated. Deletion protection, backup retention, the name and the maintenance window are free to change. When a change costs extra, the API returns a quote. With `accept_price_changes = false` the apply stops with the charge, the new monthly price and the currency; set `accept_price_changes = true` and apply again to go ahead. With `true`, the provider confirms the quoted charge and adds a warning with the amount. If the wallet doesn't cover the charge, the apply stops and asks you to top up the wallet. Downgrades and removals credit the wallet automatically.

### Restoring into a new database

`restore_from` creates the database from another Dedicated database's backups instead of starting empty. The source isn't changed, and the new database gets its own address and master password.

| Name | Required | Notes |
| --- | --- | --- |
| `source_database_id` | yes | The Dedicated database to restore from. |
| `target_time` | one of the two | Restore to this moment, as an RFC 3339 time inside the source's restorable window. |
| `backup_id` | one of the two | Restore from this backup or snapshot of the source. |
| `from_mirror` | no | Read the restore from the second backup copy. |

The source has to be a database that already exists and has backups; a database created in the same apply has no restorable point yet. Use `stackshift_dedicated_database_backups` to see the restorable window and the backup ids. `instance_class` and `storage_gb` are required as usual; `storage_gb` can't be below the source's. The engine, version, `master_username`, `database_name`, `backup_retention_days`, `storage_max_gb` and the maintenance window come from the source, so setting them (or `deletion_protection = true`, `standby` or `on_standby_loss`) together with `restore_from` stops the apply before anything is created. Set them in a later apply once the database exists.

`restore_from` is create-only. It only matters when the database is created: once the database exists, changing or removing it changes nothing and never shows a difference in the plan. Adding it to a database that already exists also changes nothing; the plan shows a warning and the first apply just records the block. The same happens after importing a database that has it in its configuration.

The target has to be given explicitly, either as a literal time or as a snapshot id, and it has to lie inside the source's restorable window. If the value you pass in is empty when the apply runs (for example the source has no backups yet), the apply stops before anything is created.

```hcl
variable "source_database_id" {
  description = "An existing Dedicated database that already has backups."
  type        = string
}

resource "stackshift_dedicated_database" "orders_before_incident" {
  name           = "orders-before-incident"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20

  restore_from = {
    source_database_id = var.source_database_id
    target_time        = "2026-10-25T14:30:00Z"
  }
}
```

### Import

```sh
terraform import stackshift_dedicated_database.orders <database_id>
```

Import reads the settings back from the API and sets `apply_changes`, `accept_price_changes`, `skip_final_snapshot` and `allow_replace` to their defaults, so the first plan after an import is clean. It can't recover `master_password`, which is only returned when the database is created, and it leaves `restore_from` empty.

### Timeouts

`timeouts = { create = "45m", update = "45m", delete = "45m" }` sets how long Terraform works on the database before giving up; each defaults to 45 minutes, and the whole operation shares that one limit. Creating waits until the database is running, except a new database that is waiting for payment, which stays pending and doesn't wait. If the wait runs out or can't tell, the apply finishes with a warning and keeps the database in state, so the next plan or refresh picks it up and a paid database is never created twice; only a database that StackShift reports as failed makes the apply fail. Updating waits until the database is running again after a size, storage or standby change, and until a size change set to apply now has finished. Deleting waits until the database is gone.

If you already have this resource in state from an earlier version of the provider, the first plan shows one update in place that changes nothing on StackShift; it records `allow_replace` and the `timeouts` block in state.

## `stackshift_dedicated_database_replica`

Adds a read replica to a Dedicated database through `POST /databases/{id}/replicas`, up to 5 per database. Each replica needs at least half the database's memory. Replicas that have caught up answer on the database's read hostname. Changing `database_id` or `instance_class` replaces the replica. Adding a replica is a paid change and follows the same `accept_price_changes` rules as `stackshift_dedicated_database`.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. |
| `instance_class` | no | Defaults to the database's size. |
| `accept_price_changes` | no | Defaults to `false`. |

Computed attributes: `id`, `status` and `site`. `timeouts = { create = "45m", delete = "45m" }` sets how long the apply waits for the replica to be running and, on destroy, to be gone. If the wait runs out, the apply finishes with a warning and keeps the replica in state; only a replica that StackShift reports as failed makes it fail.

### Import

```sh
terraform import stackshift_dedicated_database_replica.read <database_id>:<replica_id>
```

## `stackshift_database_parameter_group`

A named set of engine settings for Dedicated databases, through `/databases/parameter-groups`. A group is for one engine version (its `family`) and can be attached to any number of databases of that version with `stackshift_dedicated_database_parameter_group_attachment`. Changing the settings of a group updates every database that uses it. Groups are free.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | 1 to 60 characters, unique in your account, with no spaces at the start or end. Changes in place. |
| `family` | yes | The engine and version the group is for, such as `postgres17` or `mysql8.4`. StackShift rejects families it doesn't offer when you apply. Changes replace the group. |
| `description` | no | Changes in place. |
| `parameters` | no | A map of setting names to values. Use `stackshift_database_parameter_definitions` to see what a family accepts. Changes in place. |

Computed attributes: `id`, `version` (goes up each time the group changes) and `databases` (how many databases use it). A group that is still attached can't be deleted; destroy the attachments first.

```hcl
resource "stackshift_database_parameter_group" "tuned" {
  name   = "tuned"
  family = "postgres17"

  parameters = {
    work_mem          = "65536"
    statement_timeout = "30000"
  }
}
```

### Import

```sh
terraform import stackshift_database_parameter_group.tuned <group_id>
```

## `stackshift_dedicated_database_parameter_group_attachment`

Attaches a parameter group to a Dedicated database through `PUT /databases/{id}/parameter-group`. The database must be running, so the apply waits for a new database to finish starting. The group has to be for the database's engine and version. Changing `parameter_group_id` switches the group in place. Destroying the attachment detaches the group and returns the database to its default settings.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. Changes replace the attachment. |
| `parameter_group_id` | yes | A `stackshift_database_parameter_group` id. Changes in place. |
| `restart_to_apply` | no | Defaults to `false`. When `true`, the database restarts right after the group is attached or switched, so settings that need a restart take effect. Connections drop briefly. |
| `timeouts` | no | `create` and `update` durations. |

Computed attributes: `id` (the database id), `applied` (whether the database runs with the group's settings) and `pending_restart` (settings waiting for a restart). With `restart_to_apply = true` the apply waits until the database has restarted and every setting is applied. Turning it on for an attachment that already exists restarts the database the same way. `timeouts = { create = "45m", update = "45m" }` sets how long it waits.

### Import

```sh
terraform import stackshift_dedicated_database_parameter_group_attachment.tuned <database_id>
```

## `stackshift_dedicated_database_snapshot`

A snapshot of a Dedicated database that is kept until you delete it, through `/databases/{id}/backups`. The database must be running. The apply waits until the snapshot is ready, and destroy waits until it is gone. Use the snapshot id as `restore_from.backup_id`. Automated backups are removed by StackShift when they age out of `backup_retention_days`, so they aren't managed here.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. Changes replace the snapshot. |
| `name` | no | Up to 100 characters. Changes replace the snapshot. |

Computed attributes: `id`, `kind`, `status`, `size_bytes` and `created_at`. `timeouts = { create = "45m", delete = "45m" }` sets how long the apply waits for the snapshot to be ready and to be gone.

### Import

```sh
terraform import stackshift_dedicated_database_snapshot.before <database_id>:<snapshot_id>
```

## `stackshift_dedicated_database_restore_point`

A fast restore point of a Dedicated database, taken on demand through `/databases/{id}/restore-points` and kept until you delete it. The database must be running and fast restore points must be available for it. You can keep a limited number of your own. The apply waits until the restore point is ready. Rolling a database back to a restore point replaces its data, so Terraform doesn't do it; use the dashboard or the API for that.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. Changes replace the restore point. |
| `name` | no | Up to 100 characters. Changes replace the restore point. |

Computed attributes: `id`, `kind`, `status`, `size_gb` and `taken_at`. `timeouts = { create = "45m", delete = "45m" }` sets how long the apply waits for the restore point to be ready and to be gone.

### Import

```sh
terraform import stackshift_dedicated_database_restore_point.nightly <database_id>:<restore_point_id>
```

## `stackshift_dedicated_database_restore_schedule`

How often a Dedicated database takes automatic fast restore points, through `/databases/{id}/restore-points/schedules`. Every database has a `daily`, a `weekly` and a `monthly` schedule. Schedules you leave out keep their current settings. The database must be running. Destroying this resource only stops Terraform from managing the schedules; they keep their last settings.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. Changes replace the resource. |
| `daily`, `weekly`, `monthly` | no | `{ enabled = bool, keep = 1-30 }`. Each changes in place. |

Computed attributes: `id` (the database id) and the settings of the schedules you leave out.

### Import

```sh
terraform import stackshift_dedicated_database_restore_schedule.plan <database_id>
```

## `stackshift_dedicated_database_public_access`

Lets people outside StackShift reach a Dedicated database through a secure public address, using `/databases/{id}/public-access`. The database must be running and public access must be available for it. The apply waits until the address is active, and destroy waits until access is off. Changing any setting replaces the access, which turns it off for a moment.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. |
| `access_mode` | yes | `restricted` lets only `allowed_cidrs` connect until `expires_at`. `anywhere` lets any address connect and renews itself. |
| `allowed_cidrs` | restricted | Address ranges that may connect, written as the network address with its prefix, such as `203.0.113.0/24`, and not `0.0.0.0/0`. Terraform shows the ranges StackShift reports back. |
| `expires_at` | restricted | When access ends, as a UTC time written like `2030-01-01T00:00:00Z`, between one hour from now and the platform limit. When it passes, the access is gone and the next apply needs a new future time. For `anywhere` it is a computed renewal time. |
| `acknowledge_public_exposure` | anywhere | Set to `true` to confirm that any address can connect. |

Destroying the resource turns off the policy it created, by id, so it never turns off a newer policy that someone else created. If that policy is already gone, destroy finishes. An imported `anywhere` policy has `acknowledge_public_exposure = true`.

Computed attributes: `id`, `status`, `public_hostname` and `public_port`. `timeouts = { create = "45m", delete = "45m" }` sets how long the apply waits for access to turn on and off. Connect to `public_hostname` and `public_port` with the credentials from `stackshift_database_credentials`.

### Import

```sh
terraform import stackshift_dedicated_database_public_access.office <database_id>:<policy_id>
```

## `stackshift_dedicated_database_insights`

How long a Dedicated database keeps its performance history, through `/databases/{id}/insights`. Keeping history longer than the included days is a paid change and follows the same `accept_price_changes` rules as `stackshift_dedicated_database`. Destroying this resource returns the database to the included days.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `database_id` | yes | The Dedicated database. Changes replace the resource. |
| `retention_days` | yes | `7`, `30` or `90`. Changes in place. |
| `accept_price_changes` | no | Defaults to `false`. |

Computed attributes: `id` (the database id) and `included_days`.

### Import

```sh
terraform import stackshift_dedicated_database_insights.history <database_id>
```
