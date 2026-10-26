resource "stackshift_dedicated_database" "orders" {
  name           = "orders"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20
}

resource "stackshift_database_parameter_group" "tuned" {
  name   = "orders-tuned"
  family = "postgres17"

  parameters = {
    work_mem          = "65536"
    statement_timeout = "30000"
  }
}

resource "stackshift_dedicated_database_parameter_group_attachment" "tuned" {
  database_id        = stackshift_dedicated_database.orders.id
  parameter_group_id = stackshift_database_parameter_group.tuned.id
  restart_to_apply   = true
}

resource "stackshift_dedicated_database_snapshot" "before_migration" {
  database_id = stackshift_dedicated_database.orders.id
  name        = "before-migration"
}

resource "stackshift_dedicated_database_restore_point" "nightly" {
  database_id = stackshift_dedicated_database.orders.id
  name        = "nightly"
}

resource "stackshift_dedicated_database_restore_schedule" "orders" {
  database_id = stackshift_dedicated_database.orders.id

  daily = {
    enabled = true
    keep    = 10
  }
}

resource "stackshift_dedicated_database_public_access" "office" {
  database_id   = stackshift_dedicated_database.orders.id
  access_mode   = "restricted"
  allowed_cidrs = ["203.0.113.0/24"]
  expires_at    = "2026-12-01T00:00:00Z"
}

resource "stackshift_dedicated_database_insights" "orders" {
  database_id          = stackshift_dedicated_database.orders.id
  retention_days       = 30
  accept_price_changes = true
}

data "stackshift_dedicated_database_backups" "orders" {
  database_id = stackshift_dedicated_database.orders.id
  depends_on  = [stackshift_dedicated_database_snapshot.before_migration]
}

resource "stackshift_dedicated_database" "orders_before_incident" {
  name           = "orders-before-incident"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20

  restore_from = {
    source_database_id = stackshift_dedicated_database.orders.id
    target_time        = data.stackshift_dedicated_database_backups.orders.restorable_until
  }

  lifecycle {
    ignore_changes = [restore_from]
  }
}
