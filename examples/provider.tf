terraform {
  required_providers {
    stackshift = {
      source = "stackshiftCloud/stackshift"
    }
  }
}

provider "stackshift" {
  # endpoint  = "https://api.stackshift.cloud"
  # api_token = var.stackshift_api_token
}
