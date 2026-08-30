resource "stackshift_bucket" "uploads" {
  name             = "example-customer-uploads"
  region           = "global"
  visibility       = "private"
  access_key_label = "terraform-production"
  force_destroy    = false

  versioning_enabled     = true
  default_retention_days = 30
  encryption_mode        = "sse-kms"
}

output "s2_endpoint" {
  value = stackshift_bucket.uploads.endpoint
}

output "s2_access_key_id" {
  value = stackshift_bucket.uploads.access_key_id
}

output "s2_secret_access_key" {
  value     = stackshift_bucket.uploads.secret_access_key
  sensitive = true
}

output "s2_kms_key_id" {
  value = stackshift_bucket.uploads.kms_key_id
}
