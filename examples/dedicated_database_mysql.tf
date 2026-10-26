data "stackshift_database_instance_classes" "all" {}

resource "stackshift_dedicated_database" "shop" {
  name                  = "shop"
  engine                = "mysql"
  instance_class        = data.stackshift_database_instance_classes.all.classes[0].slug
  storage_gb            = 20
  backup_retention_days = 14
  billing               = "monthly"
}

data "stackshift_database_credentials" "shop" {
  database_id = stackshift_dedicated_database.shop.id
}

output "shop_endpoints" {
  value = [for endpoint in stackshift_dedicated_database.shop.endpoints : "${endpoint.role} ${endpoint.host}:${endpoint.port}"]
}
