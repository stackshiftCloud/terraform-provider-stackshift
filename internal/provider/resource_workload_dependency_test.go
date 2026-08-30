package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func TestWorkloadDependencyPreservesConfiguredSpelling(t *testing.T) {
	plan := workloadDependencyModel{
		Environment: types.StringValue(" Production "), Hostname: types.StringValue("DB.Example.COM."),
		Protocol: types.StringValue("TLS"), Purpose: types.StringValue(" database "),
	}
	plan.apply(client.WorkloadExternalDependency{Environment: "production", Hostname: "db.example.com", Protocol: "tls", Purpose: "database"})
	if plan.Environment.ValueString() != " Production " || plan.Hostname.ValueString() != "DB.Example.COM." || plan.Protocol.ValueString() != "TLS" {
		t.Fatalf("configured spelling was overwritten: %+v", plan)
	}
	plan.apply(client.WorkloadExternalDependency{Environment: "production", Hostname: "changed.example.com", Protocol: "tls"})
	if plan.Hostname.ValueString() != "changed.example.com" {
		t.Fatal("remote hostname drift was hidden")
	}
}

func TestMatchingDependencyCanonicalizesEndpointIdentity(t *testing.T) {
	t.Parallel()
	plan := workloadDependencyModel{
		Environment: types.StringValue(" Production "),
		Hostname:    types.StringValue("DB.Example.COM."),
		Port:        types.Int64Value(5432),
		Protocol:    types.StringValue("TLS"),
	}
	dependencies := []client.WorkloadExternalDependency{{
		ID: "dependency-1", Environment: "production", Hostname: "db.example.com",
		Port: 5432, Protocol: "tls",
	}}

	matched := matchingDependency(dependencies, plan)
	if matched == nil || matched.ID != "dependency-1" {
		t.Fatalf("matching dependency = %#v", matched)
	}
}
