terraform {
  required_providers {
    stackshift = {
      source = "stackshift/stackshift"
    }
  }
}

provider "stackshift" {
  api_token = var.stackshift_api_token
}

variable "stackshift_api_token" {
  type      = string
  sensitive = true
}

variable "hetzner_token" {
  type      = string
  sensitive = true
}

resource "stackshift_byoc_provider_connection" "hetzner" {
  provider     = "hetzner"
  display_name = "Production Hetzner project"
  token        = var.hetzner_token
}

resource "stackshift_byoc_node" "app" {
  provider_connection_id = stackshift_byoc_provider_connection.hetzner.id
  provider               = "hetzner"
  region                 = "fsn1"
  tier_slug              = "growth"
  name                   = "production-app-1"
}

resource "stackshift_byoc_volume" "data" {
  node_id    = stackshift_byoc_node.app.id
  name       = "production-app-data"
  size_gb    = 80
  mount_path = "/var/lib/stackshift-data"
  filesystem = "ext4"
}

resource "stackshift_byoc_snapshot" "baseline" {
  node_id   = stackshift_byoc_node.app.id
  volume_id = stackshift_byoc_volume.data.id
  name      = "production-app-baseline"
}

resource "stackshift_byoc_static_ip" "app" {
  node_id = stackshift_byoc_node.app.id
}
