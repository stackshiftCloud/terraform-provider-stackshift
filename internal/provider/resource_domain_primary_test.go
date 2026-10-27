package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

type domainOnlyProvider struct{ stackshiftProvider }

func (domainOnlyProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{NewDomainResource}
}

func (domainOnlyProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

type fakeDomainAPI struct {
	mu       sync.Mutex
	domains  map[string]map[string]any
	order    []string
	primary  []string
	verified map[string]bool
}

func (f *fakeDomainAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/p-1/custom-domains")
	respond := func(status int, data any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": status < 300, "data": data, "error": data})
	}
	switch {
	case r.Method == http.MethodPost && path == "/external":
		var body struct {
			Hostname  string `json:"hostname"`
			IsPrimary bool   `json:"is_primary"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := "pd-" + strings.Split(body.Hostname, ".")[0]
		f.domains[id] = map[string]any{"id": id, "project_id": "p-1", "hostname": body.Hostname, "ssl_status": "pending"}
		f.order = append(f.order, id)
		respond(http.StatusCreated, f.view(id))
	case r.Method == http.MethodGet && path == "":
		list := []any{}
		for _, id := range f.order {
			if f.domains[id] != nil {
				list = append(list, f.view(id))
			}
		}
		respond(http.StatusOK, map[string]any{"domains": list})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/primary"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/primary")
		if !f.verified[id] {
			respond(http.StatusConflict, "verify before making it the primary domain")
			return
		}
		f.primary = append(f.primary, id)
		respond(http.StatusOK, map[string]any{"message": "primary domain updated"})
	case r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/")
		if f.domains[id] == nil {
			respond(http.StatusNotFound, "project domain not found")
			return
		}
		delete(f.domains, id)
		respond(http.StatusOK, map[string]any{"message": "domain detached from project"})
	default:
		respond(http.StatusNotFound, "not found")
	}
}

func (f *fakeDomainAPI) view(id string) map[string]any {
	domain := map[string]any{}
	for key, value := range f.domains[id] {
		domain[key] = value
	}
	domain["is_primary"] = len(f.primary) > 0 && f.primary[len(f.primary)-1] == id
	domain["verification_status"] = "failed"
	domain["verification_record"] = map[string]any{"type": "TXT", "name": "_stackshift-verify." + domain["hostname"].(string), "value": "token-" + id}
	if f.verified[id] {
		domain["verification_status"] = "verified"
		delete(domain, "verification_record")
	}
	return domain
}

func (f *fakeDomainAPI) set(change func()) func() {
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		change()
	}
}

func TestDomainPrimaryChangesUpdateInPlaceAndWaitForVerification(t *testing.T) {
	api := &fakeDomainAPI{domains: map[string]map[string]any{}, verified: map[string]bool{}}
	server := httptest.NewServer(api)
	defer server.Close()
	t.Setenv("STACKSHIFT_ENDPOINT", server.URL)
	t.Setenv("STACKSHIFT_API_TOKEN", "sspat_test")
	config := func(primary bool) string {
		return fmt.Sprintf(`
resource "stackshift_domain" "shop" {
  project_id = "p-1"
  domain     = "shop.example.com"
  primary    = %t
}

resource "stackshift_domain" "blog" {
  project_id = "p-1"
  domain     = "blog.example.com"
}
`, primary)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"stackshift": providerserver.NewProtocol6WithError(&domainOnlyProvider{}),
		},
		Steps: []resource.TestStep{
			{
				Config: config(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stackshift_domain.shop", "primary", "true"),
					resource.TestCheckResourceAttr("stackshift_domain.shop", "is_primary", "false"),
					resource.TestCheckResourceAttr("stackshift_domain.shop", "verification_txt_name", "_stackshift-verify.shop.example.com"),
					resource.TestCheckResourceAttr("stackshift_domain.shop", "verification_txt_value", "token-pd-shop"),
				),
			},
			{
				PreConfig: api.set(func() { api.verified["pd-shop"] = true }),
				Config:    config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("stackshift_domain.shop", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("stackshift_domain.blog", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("stackshift_domain.shop", "is_primary", "true"),
					resource.TestCheckResourceAttr("stackshift_domain.shop", "verification_txt_name", ""),
				),
			},
			{
				PreConfig: api.set(func() { api.verified["pd-blog"] = true; api.primary = append(api.primary, "pd-blog") }),
				Config:    config(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("stackshift_domain.shop", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("stackshift_domain.shop", "is_primary", "true"),
			},
			{
				Config: config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("stackshift_domain.shop", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("stackshift_domain.shop", "primary", "false"),
			},
			{
				PreConfig: api.set(func() { delete(api.domains, "pd-blog") }),
				Config:    config(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("stackshift_domain.blog", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
	if got := strings.Join(api.primary, ","); got != "pd-shop,pd-blog,pd-shop" {
		t.Fatalf("primary calls = %s", got)
	}
}
