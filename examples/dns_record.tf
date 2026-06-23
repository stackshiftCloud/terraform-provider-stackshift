resource "stackshift_dns_record" "www" {
  domain_id = "registered-domain-uuid"
  type      = "CNAME"
  name      = "www"
  value     = "example.stackshift.cloud"
  ttl       = 3600
}
