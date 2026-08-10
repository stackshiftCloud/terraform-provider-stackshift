# Terraform Provider for StackShift

Use this provider to manage StackShift projects, environment variables, databases, S2 object-storage buckets, domains, DNS records, build/deployment actions, compute instances, agency resources, and runbooks from Terraform or OpenTofu.

S2 returns a bucket secret access key only when the bucket is created. The provider marks it sensitive, but Terraform still stores it in state. Use encrypted remote state with tightly scoped access.

## Install

After the provider is published to the Terraform Registry:

```hcl
terraform {
  required_providers {
    stackshift = {
      source  = "stackshiftCloud/stackshift"
      version = "~> 0.1"
    }
  }
}
```

```hcl
provider "stackshift" {
  endpoint  = "https://api.stackshift.cloud"
  api_token = var.stackshift_api_token
}
```

You can also set credentials with environment variables:

```bash
export STACKSHIFT_API_TOKEN="sspat_..."
export STACKSHIFT_ENDPOINT="https://api.stackshift.cloud"
```

## Local development

Build the provider:

```bash
go build -o terraform-provider-stackshift
```

Point Terraform at the local build with `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/stackshiftCloud/stackshift" = "/path/to/terraform-provider-stackshift"
  }

  direct {}
}
```

## Publishing

1. Create a public GitHub repository named `terraform-provider-stackshift`.
2. Add an RSA or DSA GPG signing key to Terraform Registry under the StackShift namespace.
3. Add `GPG_PRIVATE_KEY` and `PASSPHRASE` repository secrets in GitHub.
4. Push a SemVer tag such as `v0.1.0`.
5. GitHub Actions runs GoReleaser and creates a signed GitHub release.
6. In Terraform Registry, choose Publish > Provider and select the GitHub repository.

Future releases are detected from GitHub release events after the Registry webhook is installed.
