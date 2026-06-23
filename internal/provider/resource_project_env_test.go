package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectEnvResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectEnvConfig(name, "production", "value1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stackshift_project_env.test", "environment", "production"),
					resource.TestCheckResourceAttr("stackshift_project_env.test", "variables.KEY_ONE", "value1"),
				),
			},
			{
				Config: testAccProjectEnvConfig(name, "production", "value2"),
				Check:  resource.TestCheckResourceAttr("stackshift_project_env.test", "variables.KEY_ONE", "value2"),
			},
		},
	})
}

func testAccProjectEnvConfig(name, environment, value string) string {
	return fmt.Sprintf(`
provider "stackshift" {}

resource "stackshift_project" "test" {
  name    = %[1]q
  runtime = "nodejs"
  region  = %[2]q
}

resource "stackshift_project_env" "test" {
  project_id  = stackshift_project.test.id
  environment = %[3]q
  variables = {
    KEY_ONE = %[4]q
  }
}
`, name, envOr("STACKSHIFT_TEST_REGION", "global"), environment, value)
}
