resource "stackshift_mail_domain" "sending" {
  domain           = "mail.example.com"
  verify_on_create = true
}

resource "stackshift_mail_webhook" "delivery_events" {
  url         = "https://hooks.example.com/stackshift-mail"
  description = "Production recipient delivery events"
  event_types = [
    "mail.message.mta_accepted",
    "mail.delivery.delivered",
    "mail.delivery.delayed",
    "mail.message.bounced",
    "mail.message.complained",
  ]
}

output "mail_webhook_secret" {
  value     = stackshift_mail_webhook.delivery_events.secret
  sensitive = true
}
