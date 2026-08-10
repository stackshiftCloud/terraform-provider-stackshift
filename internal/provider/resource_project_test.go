package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	updated := name + "-renamed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("stackshift_project.test", "id"),
					resource.TestCheckResourceAttr("stackshift_project.test", "name", name),
					resource.TestCheckResourceAttr("stackshift_project.test", "runtime", "nodejs"),
					resource.TestCheckResourceAttrSet("stackshift_project.test", "status"),
				),
			},
			{
				Config: testAccProjectConfig(updated),
				Check:  resource.TestCheckResourceAttr("stackshift_project.test", "name", updated),
			},
			{
				ResourceName:      "stackshift_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectConfig(name string) string {
	return fmt.Sprintf(`
provider "stackshift" {}

resource "stackshift_project" "test" {
  name    = %[1]q
  runtime = "nodejs"
  region  = %[2]q
}
`, name, envOr("STACKSHIFT_TEST_REGION", "global"))
}
