# Resources

## `stackshift_project`

Manages a StackShift project through `/api/v1/projects`.

Normal StackShift-managed deployments do not require a node ID. Leave placement to StackShift unless a separate resource explicitly exposes placement.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `name` | yes | Project display name. |
| `runtime` | yes | Runtime identifier accepted by StackShift. |
| `region` | yes | StackShift region. |
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
| `type` | yes | `postgres`, `mysql`, or `redis`. |
| `version` | yes | Version accepted by StackShift. |
| `size_gb` | yes | Managed database disk size. |
| `target_node_id` | no | Optional explicit node placement target. |

### Import

```sh
terraform import stackshift_database.postgres <database_id>
```

## `stackshift_domain`

Maps an external/custom domain to a project.

### Arguments

| Name | Required | Notes |
| --- | --- | --- |
| `project_id` | yes | Replaces the resource if changed. |
| `domain` | yes | Domain hostname to attach. |
| `verify_on_create` | no | Queues StackShift verification after creation. |

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

- `provider = "hetzner"` or `"digitalocean"` requires `token`. Use a provider-scoped read/write token.
- `provider = "aws"` requires `role_arn` and `region`. StackShift returns `external_id`; put that exact value in the IAM role trust policy. AWS access keys are not accepted.
- `provider = "azure"` requires `azure_tenant_id`, `azure_subscription_id`, and `azure_client_id`. Configure the issuer, subject, and audience returned by StackShift on the Azure federated credential. Client secrets are not accepted.
- `missing_permissions` contains the exact failed permission checks when validation fails.

Provider credentials are sensitive but remain Terraform inputs, so protect remote state with encryption and strict access control. Imported connections can be read without credentials, but credentials must be supplied before changing or revalidating them.

## `stackshift_byoc_node`

Provisions a provider-backed node and waits for the durable provisioning operation to become `active`. The provider resource is discovered by StackShift tags after ambiguous provider responses, so a repeated apply with the same state does not create a duplicate instance. Destroy starts the durable deletion saga and waits until the provider confirms deletion. A `409` deletion blocker is returned to Terraform when projects, stacks, databases, volumes, or dependent snapshots remain.

Required fields are `provider_connection_id`, `provider`, `region`, `tier_slug`, and `name`. When omitted, `idempotency_key` is deterministically derived from the immutable resource identity and then persisted, so a Terraform process crash cannot turn a retry into a second provider resource. Read-only fields expose operation progress, bootstrap state, enrollment state, overlay address, public address, and failure details.

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

Computed attributes include `endpoint`, `access_key_id`, `secret_access_key`, `object_count`, `size_bytes`, `created_at`, and `updated_at`.

### Import

```sh
terraform import stackshift_bucket.uploads <bucket_id>
```
