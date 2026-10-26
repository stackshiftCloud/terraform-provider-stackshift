variable "public_access_expires_at" {
  description = "When the office's access ends, as a UTC time between one hour from now and the platform limit, such as 2030-01-01T00:00:00Z."
  type        = string
}

variable "source_database_id" {
  description = "An existing Dedicated database that already has backups. A database created in this same apply has no restorable point yet."
  type        = string
}

variable "restore_time" {
  description = "The moment to restore to, as a UTC time inside the source database's restorable window, such as 2030-01-01T00:00:00Z."
  type        = string
}

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
  expires_at    = var.public_access_expires_at
}

resource "stackshift_dedicated_database_insights" "orders" {
  database_id          = stackshift_dedicated_database.orders.id
  retention_days       = 30
  accept_price_changes = true
}

data "stackshift_dedicated_database_backups" "source" {
  database_id = var.source_database_id
}

output "source_restorable_window" {
  value = "${data.stackshift_dedicated_database_backups.source.restorable_from} to ${data.stackshift_dedicated_database_backups.source.restorable_until}"
}

resource "stackshift_dedicated_database" "restored" {
  name           = "orders-restored"
  instance_class = "s-2vcpu-4gb"
  storage_gb     = 20

  restore_from = {
    source_database_id = var.source_database_id
    target_time        = var.restore_time
  }
}
