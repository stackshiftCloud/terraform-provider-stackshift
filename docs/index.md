# StackShift Provider

The StackShift Terraform/OpenTofu provider lets Terraform manage StackShift resources by calling the existing StackShift REST API. It runs locally in Terraform, sends authenticated HTTPS requests to StackShift, and stores the resulting resource IDs in Terraform state.

It is not a remote Terraform runner and it does not store Terraform state on StackShift.

## How It Works

Terraform loads the provider binary, reads the provider configuration, and creates a StackShift API client. Each resource method maps directly to an API endpoint:

- `stackshift_project` calls `/api/v1/projects`
- `stackshift_project_env` calls `/api/v1/projects/{projectID}/env`
- `stackshift_database` calls `/api/v1/projects/{projectID}/databases` and `/api/v1/databases/{databaseID}`
- `stackshift_domain` calls `/api/v1/projects/{projectID}/domains`
- `stackshift_dns_record` calls `/api/v1/domains/{domainID}/dns`
- `stackshift_build_action` and `stackshift_deployment_action` trigger explicit operational actions
- `stackshift_compute_instance` and `stackshift_compute_action` manage VM/VPS compute resources and actions
- `stackshift_agency_client` and `stackshift_agency_resource_assignment` manage agency records
- `stackshift_runbook` and `stackshift_runbook_execution` manage runbooks and execution requests

The API response envelope is decoded from `success`, `status_code`, `message`, and `data`. A `404` is mapped to Terraform drift handling so resources removed outside Terraform are removed from state on the next read.

Action resources create operational requests. Removing an action resource from Terraform state does not undo the operation that already ran.

## Authentication

Create a StackShift API token with the `sspat_` prefix and pass it either in provider configuration or through the environment.

```hcl
provider "stackshift" {
  api_token = var.stackshift_api_token
}
```

Environment variable:

```sh
export STACKSHIFT_API_TOKEN="sspat_..."
```

Current server-side token scopes are free-form and default to `*` when no scopes are supplied. Until StackShift exposes finer-grained scopes for these APIs, use a full-access API token.

## Endpoint

The provider defaults to production:

```text
https://api.stackshift.cloud
```

Override it for staging or local development:

```hcl
provider "stackshift" {
  endpoint = "https://api.stackshift.cloud"
}
```

Environment variable:

```sh
export STACKSHIFT_ENDPOINT="https://api.stackshift.cloud"
```

## Local Development Install

Build the provider:

```sh
cd /Users/jessejosiah/Startup/terraform-provider-stackshift
go build -o terraform-provider-stackshift
```

Add a Terraform CLI config file at `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/stackshiftCloud/stackshift" = "/Users/jessejosiah/Startup/terraform-provider-stackshift"
  }
  direct {}
}
```

Then use the provider in a Terraform project:

```hcl
terraform {
  required_providers {
    stackshift = {
      source = "stackshiftCloud/stackshift"
    }
  }
}
```

## Minimal Example

```hcl
resource "stackshift_project" "api" {
  name    = "example-api"
  runtime = "node"
  region  = "us-east"
  port    = 3000
}

resource "stackshift_project_env" "production" {
  project_id  = stackshift_project.api.id
  environment = "production"

  variables = {
    NODE_ENV = "production"
    API_KEY  = var.api_key
  }
}
```

Run:

```sh
terraform init
terraform plan
terraform apply
```

## Generated Docs Note

These docs are currently maintained by hand because `tfplugindocs` is not installed in this workspace. Once available, run:

```sh
tfplugindocs generate
```
