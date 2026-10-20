data "stackshift_regions" "apps" {
  product = "apps"
}

data "stackshift_app_sizes" "picker" {
  listed_only = true
}

output "default_app_region" {
  value = data.stackshift_regions.apps.default_region
}

output "app_size_slugs" {
  value = [for size in data.stackshift_app_sizes.picker.sizes : size.slug]
}
