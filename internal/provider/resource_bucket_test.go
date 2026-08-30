package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBucketSchemaKeepsKMSKeyPlatformManaged(t *testing.T) {
	t.Parallel()

	var response resource.SchemaResponse
	(&bucketResource{}).Schema(context.Background(), resource.SchemaRequest{}, &response)

	encryption, ok := response.Schema.Attributes["encryption_mode"].(schema.StringAttribute)
	if !ok || !encryption.Optional || !encryption.Computed {
		t.Fatalf("encryption_mode = %#v", response.Schema.Attributes["encryption_mode"])
	}
	if encryption.Default != nil {
		t.Fatal("encryption_mode must preserve remote state when omitted")
	}
	kmsKey, ok := response.Schema.Attributes["kms_key_id"].(schema.StringAttribute)
	if !ok || !kmsKey.Computed || kmsKey.Optional || kmsKey.Required {
		t.Fatalf("kms_key_id = %#v", response.Schema.Attributes["kms_key_id"])
	}
}

func TestBucketSettingsRequestDefaultsOnlyUnsetCreateValues(t *testing.T) {
	t.Parallel()

	request := (&bucketModel{}).settingsRequest()
	if request.EncryptionMode != "none" {
		t.Fatalf("encryption mode = %q, want none", request.EncryptionMode)
	}
	if request.WebsiteIndex != "index.html" {
		t.Fatalf("website index = %q, want index.html", request.WebsiteIndex)
	}
}

func TestBucketSettingsRequestUsesPlatformManagedKMS(t *testing.T) {
	t.Parallel()

	quota := int64(20 << 30)
	model := bucketModel{
		Versioning:     types.BoolValue(true),
		QuotaBytes:     types.Int64Value(quota),
		RetentionDays:  types.Int64Value(30),
		TierAfterDays:  types.Int64Value(7),
		WebsiteEnabled: types.BoolValue(false),
		WebsiteIndex:   types.StringValue("index.html"),
		WebsiteError:   types.StringValue("error.html"),
		EncryptionMode: types.StringValue("sse-kms"),
		KMSKeyID:       types.StringValue("customer-key-must-not-be-sent"),
	}

	request := model.settingsRequest()
	if request.EncryptionMode != "sse-kms" {
		t.Fatalf("encryption mode = %q", request.EncryptionMode)
	}
	if request.QuotaBytes == nil || *request.QuotaBytes != quota {
		t.Fatalf("quota = %#v", request.QuotaBytes)
	}
	if request.DefaultRetentionDays != 30 || request.TierAfterDays != 7 {
		t.Fatalf("retention = %d, tier = %d", request.DefaultRetentionDays, request.TierAfterDays)
	}
}
