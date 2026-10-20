package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type catalogOnlyProvider struct{ stackshiftProvider }

func (catalogOnlyProvider) Resources(context.Context) []func() frameworkresource.Resource { return nil }

func (catalogOnlyProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewRegionsDataSource, NewAppSizesDataSource}
}

const catalogRegionsBody = `{"success":true,"data":{"default_region":"hel1","regions":[
{"id":"hel1","name":"Helsinki, Finland","city":"Helsinki","country":"Finland","country_code":"FI","default":true,"status":"available",
 "products":{"apps":{"available":true,"tiers":["shared","isolated"]},"databases":{"available":true,"tiers":["starter"]},"vps":{"available":false,"tiers":[]}}},
{"id":"fsn1","name":"Falkenstein, Germany","city":"Falkenstein","country":"Germany","country_code":"DE","default":false,"status":"available",
 "products":{"apps":{"available":false,"tiers":[]},"databases":{"available":false,"tiers":[]},"vps":{"available":true,"tiers":[]}}}]}}`

const catalogSizesBody = `{"success":true,"data":{"version":1,"currency":"NGN","sizes":[
{"slug":"standard","name":"Standard","description":"Websites and small APIs","memory_mb":512,"cpu_millicores":500,"listed":true,"recommended":true,"price_ngn_monthly":null,"included_in_plan":true,"min_plan":"free","plans":["free","ignite","pro","enterprise"]},
{"slug":"cms","name":"CMS","description":"","memory_mb":1024,"cpu_millicores":1000,"listed":false,"recommended":false,"price_ngn_monthly":2500,"included_in_plan":false,"min_plan":"ignite","plans":["ignite","pro","enterprise"]}],
"plans":[{"slug":"enterprise","name":"Enterprise","max_memory_mb":8192,"max_cpu_millicores":-1,"app_tier":"isolated","default_size":"heavy"}]}}`

func TestCatalogDataSourcesReadRegionsAndAppSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/regions":
			_, _ = w.Write([]byte(catalogRegionsBody))
		case "/api/v1/app-sizes":
			_, _ = w.Write([]byte(catalogSizesBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("STACKSHIFT_ENDPOINT", server.URL)
	t.Setenv("STACKSHIFT_API_TOKEN", "sspat_test")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"stackshift": providerserver.NewProtocol6WithError(&catalogOnlyProvider{}),
		},
		Steps: []resource.TestStep{{
			Config: `
data "stackshift_regions" "all" {}

data "stackshift_regions" "databases" {
  product = "databases"
}

data "stackshift_app_sizes" "all" {}

data "stackshift_app_sizes" "picker" {
  listed_only = true
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.stackshift_regions.all", "default_region", "hel1"),
				resource.TestCheckResourceAttr("data.stackshift_regions.all", "regions.#", "2"),
				resource.TestCheckResourceAttr("data.stackshift_regions.all", "regions.1.vps_available", "true"),
				resource.TestCheckResourceAttr("data.stackshift_regions.all", "regions.1.app_tiers.#", "0"),
				resource.TestCheckResourceAttr("data.stackshift_regions.databases", "regions.#", "1"),
				resource.TestCheckResourceAttr("data.stackshift_regions.databases", "regions.0.id", "hel1"),
				resource.TestCheckResourceAttr("data.stackshift_regions.databases", "regions.0.database_tiers.0", "starter"),
				resource.TestCheckResourceAttr("data.stackshift_regions.databases", "regions.0.app_tiers.1", "isolated"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.all", "currency", "NGN"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.all", "sizes.#", "2"),
				resource.TestCheckNoResourceAttr("data.stackshift_app_sizes.all", "sizes.0.price_ngn_monthly"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.all", "sizes.1.price_ngn_monthly", "2500"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.all", "plans.0.max_cpu_millicores", "-1"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.picker", "sizes.#", "1"),
				resource.TestCheckResourceAttr("data.stackshift_app_sizes.picker", "sizes.0.slug", "standard"),
			),
		}},
	})
}
