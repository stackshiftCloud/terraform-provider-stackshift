package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainResource(t *testing.T) {
	domain := os.Getenv("STACKSHIFT_TEST_DOMAIN")
	if domain == "" {
		t.Skip("STACKSHIFT_TEST_DOMAIN not set")
	}
	name := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainConfig(name, domain),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("stackshift_domain.test", "id"),
					resource.TestCheckResourceAttr("stackshift_domain.test", "domain", domain),
				),
			},
		},
	})
}

func TestAccDNSRecordResource(t *testing.T) {
	domainID := os.Getenv("STACKSHIFT_TEST_REGISTERED_DOMAIN_ID")
	if domainID == "" {
		t.Skip("STACKSHIFT_TEST_REGISTERED_DOMAIN_ID not set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDNSRecordConfig(domainID, "203.0.113.10"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("stackshift_dns_record.test", "id"),
					resource.TestCheckResourceAttr("stackshift_dns_record.test", "value", "203.0.113.10"),
				),
			},
			{
				Config: testAccDNSRecordConfig(domainID, "203.0.113.20"),
				Check:  resource.TestCheckResourceAttr("stackshift_dns_record.test", "value", "203.0.113.20"),
			},
		},
	})
}

func testAccDomainConfig(name, domain string) string {
	return fmt.Sprintf(`
provider "stackshift" {}

resource "stackshift_project" "test" {
  name    = %[1]q
  runtime = "nodejs"
  region  = %[2]q
}

resource "stackshift_domain" "test" {
  project_id = stackshift_project.test.id
  domain     = %[3]q
}
`, name, envOr("STACKSHIFT_TEST_REGION", "global"), domain)
}

func testAccDNSRecordConfig(domainID, value string) string {
	return fmt.Sprintf(`
provider "stackshift" {}

resource "stackshift_dns_record" "test" {
  domain_id = %[1]q
  type      = "A"
  name      = "tf-acc"
  value     = %[2]q
  ttl       = 3600
}
`, domainID, value)
}
