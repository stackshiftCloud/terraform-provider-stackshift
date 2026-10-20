data "stackshift_database_instance_classes" "all" {}

resource "stackshift_dedicated_database" "orders" {
  name                  = "orders"
  instance_class        = data.stackshift_database_instance_classes.all.classes[0].slug
  storage_gb            = 20
  backup_retention_days = 14
  deletion_protection   = true
  standby               = "sync"
  accept_price_changes  = true

  maintenance_window = {
    day_of_week  = 0
    start_minute = 120
  }
}

resource "stackshift_dedicated_database_replica" "read" {
  database_id          = stackshift_dedicated_database.orders.id
  accept_price_changes = true
}

output "orders_host" {
  value = stackshift_dedicated_database.orders.host
}

output "orders_master_password" {
  value     = stackshift_dedicated_database.orders.master_password
  sensitive = true
}
