package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

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
