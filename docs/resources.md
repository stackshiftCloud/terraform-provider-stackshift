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
