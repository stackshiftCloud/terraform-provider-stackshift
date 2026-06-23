package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatabaseResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("stackshift_database.test", "id"),
					resource.TestCheckResourceAttr("stackshift_database.test", "type", "postgres"),
					resource.TestCheckResourceAttr("stackshift_database.test", "version", "16"),
					resource.TestCheckResourceAttr("stackshift_database.test", "size_gb", "1"),
					resource.TestCheckResourceAttrPair(
						"data.stackshift_database_credentials.test", "database_id",
						"stackshift_database.test", "id",
					),
					resource.TestCheckResourceAttrSet("data.stackshift_database_credentials.test", "connection_string"),
				),
			},
		},
	})
}

func testAccDatabaseConfig(name string) string {
	return fmt.Sprintf(`
provider "stackshift" {}

resource "stackshift_project" "test" {
  name    = %[1]q
  runtime = "nodejs"
  region  = %[2]q
}

resource "stackshift_database" "test" {
  project_id = stackshift_project.test.id
  name       = %[1]q
  type       = "postgres"
  version    = "16"
  size_gb    = 1
}

data "stackshift_database_credentials" "test" {
  database_id = stackshift_database.test.id
}
`, name, envOr("STACKSHIFT_TEST_REGION", "global"))
}
