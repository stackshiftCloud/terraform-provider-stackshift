package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type payloadDiagnostics struct {
	errors int
}

func (d *payloadDiagnostics) AddError(_, _ string) {
	d.errors++
}

func TestAssetBucketPayloadAcceptsOmittedSets(t *testing.T) {
	diagnostics := &payloadDiagnostics{}
	payload := assetBucketPayload(t.Context(), assetBucketModel{
		AllowedMimeTypes: types.SetNull(types.StringType),
		CORSOrigins:      types.SetNull(types.StringType),
		AllowedOrigins:   types.SetNull(types.StringType),
	}, diagnostics)

	if diagnostics.errors != 0 {
		t.Fatalf("omitted sets produced %d diagnostics", diagnostics.errors)
	}
	for _, name := range []string{"allowed_mime_types", "cors_origins", "allowed_origins"} {
		values, ok := payload[name].([]string)
		if !ok || len(values) != 0 {
			t.Fatalf("%s = %#v, want an empty string slice", name, payload[name])
		}
	}
}

func TestAssetContentPolicyPayloadAcceptsOmittedMimeTypes(t *testing.T) {
	diagnostics := &payloadDiagnostics{}
	payload := assetContentPolicyPayload(t.Context(), assetContentPolicyModel{
		AllowedMimeTypes: types.SetNull(types.StringType),
	}, diagnostics)

	if diagnostics.errors != 0 {
		t.Fatalf("omitted MIME types produced %d diagnostics", diagnostics.errors)
	}
	values, ok := payload["allowed_mime_types"].([]string)
	if !ok || len(values) != 0 {
		t.Fatalf("allowed_mime_types = %#v, want an empty string slice", payload["allowed_mime_types"])
	}
}

func TestAssetBucketNameRequiresReplacement(t *testing.T) {
	var response resource.SchemaResponse
	(&assetBucketResource{}).Schema(context.Background(), resource.SchemaRequest{}, &response)

	attribute, ok := response.Schema.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("name attribute has type %T", response.Schema.Attributes["name"])
	}
	if len(attribute.PlanModifiers) != 1 {
		t.Fatalf("name plan modifiers = %d, want 1", len(attribute.PlanModifiers))
	}
}

func TestStaticIPAbsentIncludesDeletedInventory(t *testing.T) {
	tests := []struct {
		name    string
		address *client.BYOCStaticIP
		absent  bool
	}{
		{name: "missing", absent: true},
		{name: "empty provider ID", address: &client.BYOCStaticIP{}, absent: true},
		{name: "deleted", address: &client.BYOCStaticIP{ProviderResourceID: "ip-1", Status: "deleted"}, absent: true},
		{name: "deleted mixed case", address: &client.BYOCStaticIP{ProviderResourceID: "ip-1", Status: " Deleted "}, absent: true},
		{name: "assigned", address: &client.BYOCStaticIP{ProviderResourceID: "ip-1", Status: "assigned"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := staticIPAbsent(test.address); got != test.absent {
				t.Fatalf("staticIPAbsent() = %t, want %t", got, test.absent)
			}
		})
	}
}
