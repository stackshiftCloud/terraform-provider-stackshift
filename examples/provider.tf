terraform {
  required_providers {
    stackshift = {
      source = "stackshift/stackshift"
    }
  }
}

provider "stackshift" {
  # endpoint  = "https://api.stackshift.cloud"
  # api_token = var.stackshift_api_token
}
