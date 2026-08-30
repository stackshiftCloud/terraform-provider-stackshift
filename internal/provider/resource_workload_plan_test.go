package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWorkloadPlansPreserveKnownConfiguration(t *testing.T) {
	ctx := context.Background()
	for name, target := range map[string]resource.Resource{
		"dependency": NewWorkloadDependencyResource(),
		"grant":      NewWorkloadEgressGrantResource(),
	} {
		t.Run(name, func(t *testing.T) {
			var response resource.SchemaResponse
			target.Schema(ctx, resource.SchemaRequest{}, &response)
			for field, value := range map[string]string{
				"environment": " Production ",
				"hostname":    "API.Example.COM.",
				"protocol":    "TLS",
				"expires_at":  "2026-09-01T01:00:00.123456789+01:00",
			} {
				attribute, ok := response.Schema.Attributes[field].(schema.StringAttribute)
				if !ok {
					continue
				}
				t.Run(field, func(t *testing.T) {
					configured := types.StringValue(value)
					result := planmodifier.StringResponse{PlanValue: configured}
					for _, modifier := range attribute.PlanModifiers {
						modifier.PlanModifyString(ctx, planmodifier.StringRequest{
							ConfigValue: configured, PlanValue: result.PlanValue,
						}, &result)
					}
					if result.Diagnostics.HasError() || !result.PlanValue.Equal(configured) {
						t.Fatalf("known configuration changed: %s -> %s (%v)", configured, result.PlanValue, result.Diagnostics)
					}
				})
			}
		})
	}
}
