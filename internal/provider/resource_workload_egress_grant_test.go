package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func TestWorkloadEgressGrantIdempotencyKeyIsStableForRetries(t *testing.T) {
	t.Parallel()
	plan := workloadEgressGrantModel{
		ProjectID: types.StringValue("project-1"), Environment: types.StringValue("production"),
		Protocol: types.StringValue("tls"), Hostname: types.StringValue("api.example.com"),
		PortStart: types.Int64Value(443), PortEnd: types.Int64Value(443),
		Purpose: types.StringValue("payment API"), BroadPublicEgress: types.BoolValue(false),
		ExpiresAt: types.StringValue("2026-09-01T00:00:00Z"),
	}

	first := workloadEgressGrantIdempotencyKey(plan)
	second := workloadEgressGrantIdempotencyKey(plan)
	if second != first {
		t.Fatalf("retry key changed from %q to %q", first, second)
	}
	plan.Purpose = types.StringValue("different request")
	if changed := workloadEgressGrantIdempotencyKey(plan); changed == first {
		t.Fatalf("different grant request reused key %q", changed)
	}
}

func TestWorkloadGrantPreservesEquivalentConfiguration(t *testing.T) {
	host := "api.example.com"
	expires := "2026-09-01T01:00:00.123456789+01:00"
	plan := workloadEgressGrantModel{
		Environment: types.StringValue("Production"), Protocol: types.StringValue("TLS"),
		Hostname: types.StringValue("API.Example.COM."), ExpiresAt: types.StringValue(expires),
	}
	plan.apply(client.WorkloadEgressGrant{Environment: "production", Protocol: "tls", Hostname: &host, ExpiresAt: canonicalRFC3339(expires)})
	if plan.Environment.ValueString() != "Production" || plan.Protocol.ValueString() != "TLS" || plan.Hostname.ValueString() != "API.Example.COM." || plan.ExpiresAt.ValueString() != expires {
		t.Fatalf("configured values were overwritten: %+v", plan)
	}
	unknown := workloadEgressGrantModel{Hostname: types.StringUnknown(), Protocol: types.StringUnknown()}
	unknown.apply(client.WorkloadEgressGrant{Protocol: "tls", Hostname: &host})
	if unknown.Protocol.ValueString() != "tls" || unknown.Hostname.ValueString() != host {
		t.Fatal("computed destination was not populated")
	}
}

func TestWorkloadEgressGrantIdempotencyKeyHonorsConfiguredValue(t *testing.T) {
	plan := workloadEgressGrantModel{IdempotencyKey: types.StringValue("operator-key")}
	got := workloadEgressGrantIdempotencyKey(plan)
	if got != "operator-key" {
		t.Fatalf("idempotency key = %q, want operator-key", got)
	}
}

func TestCanonicalRFC3339PreservesMicrosecondInstant(t *testing.T) {
	got := canonicalRFC3339("2026-09-01T01:00:00.123456789+01:00")
	if got != "2026-09-01T00:00:00.123456Z" {
		t.Fatalf("canonical expiry = %q", got)
	}
}
