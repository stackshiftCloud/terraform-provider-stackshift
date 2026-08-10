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

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
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
