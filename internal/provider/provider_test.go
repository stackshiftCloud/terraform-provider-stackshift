package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestProviderRegistersBucketResource(t *testing.T) {
	t.Parallel()

	for _, factory := range (&stackshiftProvider{}).Resources(context.Background()) {
		var response resource.MetadataResponse
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "stackshift"}, &response)
		if response.TypeName == "stackshift_bucket" {
			return
		}
	}
	t.Fatal("stackshift_bucket is not registered")
}

func TestProviderRegistersMailInfrastructureResources(t *testing.T) {
	t.Parallel()

	want := map[string]bool{
		"stackshift_mail_domain":  false,
		"stackshift_mail_webhook": false,
	}
	for _, factory := range (&stackshiftProvider{}).Resources(context.Background()) {
		var response resource.MetadataResponse
		factory().Metadata(
			context.Background(),
			resource.MetadataRequest{ProviderTypeName: "stackshift"},
			&response,
		)
		if _, ok := want[response.TypeName]; ok {
			want[response.TypeName] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("%s is not registered", name)
		}
	}
}

func TestProviderSchemaLoadsInTerraform(t *testing.T) {
	t.Parallel()

	server, err := providerserver.NewProtocol6WithError(New())()
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("%s: %s", diagnostic.Summary, diagnostic.Detail)
		}
	}
	for _, name := range []string{"stackshift_byoc_provider_connection", "stackshift_byoc_node", "stackshift_byoc_volume", "stackshift_byoc_snapshot"} {
		resource, ok := response.ResourceSchemas[name]
		if !ok {
			t.Fatalf("%s schema missing", name)
		}
		found := false
		for _, attribute := range resource.Block.Attributes {
			if attribute.Name == "provider" {
				t.Errorf("%s declares the reserved provider attribute", name)
			}
			found = found || attribute.Name == "cloud_provider"
		}
		if !found {
			t.Errorf("%s has no cloud_provider attribute", name)
		}
	}
}

var testAccProtoV6ProviderFactories =map[string]func() (tfprotov6.ProviderServer, error){
	"stackshift": providerserver.NewProtocol6WithError(New()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("STACKSHIFT_API_TOKEN") == "" {
		t.Fatal("STACKSHIFT_API_TOKEN must be set for acceptance tests")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
