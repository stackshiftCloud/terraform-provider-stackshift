variable "asset_id" {
  type        = string
  description = "An Asset ID produced by an SDK, CLI, or application upload."
}

variable "asset_job_id" {
  type        = string
  description = "A durable Assets processing job ID."
}

resource "stackshift_asset_domain" "media" {
  domain = "media.example.com"
  verify = false # Set true after publishing the computed verification record.
}

resource "stackshift_asset_bucket" "public_media" {
  name                  = "public-media"
  default_visibility    = "public"
  replication_policy    = "global-3"
  home_region           = "lagos"
  allowed_mime_types    = ["image/avif", "image/jpeg", "image/png", "image/webp"]
  cors_origins          = ["https://www.example.com"]
  allowed_origins       = ["https://www.example.com"]
  custom_domain_id      = stackshift_asset_domain.media.id
  max_object_bytes      = 52428800
  lifecycle_policy_json = jsonencode({ retain_versions = 10 })
}

resource "stackshift_asset_lifecycle_rule" "old_versions" {
  name     = "expire-old-versions"
  prefix   = "campaigns/"
  action   = "expire_versions"
  age_days = 90
  enabled  = true
}

resource "stackshift_asset_webhook" "pipeline" {
  url         = "https://hooks.example.com/stackshift/assets"
  event_types = ["asset.ready", "asset.quarantined", "asset.failed"]
}

resource "stackshift_asset_content_policy" "default" {
  allowed_mime_types = ["image/avif", "image/jpeg", "image/png", "image/webp", "video/mp4"]
  max_image_bytes    = 52428800
  max_video_bytes    = 2147483648
  max_other_bytes    = 10485760
  require_scan       = true
}

resource "stackshift_asset_transformation" "card" {
  name = "product-card"
  spec = "w_640,h_640,c_fit,f_auto,q_84,g_centre,a_first"
}

# Binary ingest and durable job creation remain application/SDK operations.
# Terraform can read their immutable result and current processing state.
data "stackshift_asset" "uploaded" {
  id = var.asset_id
}

data "stackshift_asset_job" "processing" {
  id = var.asset_job_id
}

data "stackshift_asset_analytics" "current" {}
