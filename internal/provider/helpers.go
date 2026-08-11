package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func deterministicIdempotencyKey(prefix string, identity ...string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(prefix))
	for _, value := range identity {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strings.TrimSpace(value)))
	}
	return prefix + "-" + hex.EncodeToString(hash.Sum(nil))
}

func configuredClient(data any, diags *diag.Diagnostics) *client.Client {
	c, ok := data.(*client.Client)
	if !ok || c == nil {
		diags.AddError("Provider not configured", "Expected a configured StackShift client.")
		return nil
	}
	return c
}

func stringValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

func stringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func intPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func timeString(t time.Time) types.String {
	if t.IsZero() {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

func optionalTimeString(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return timeString(t.UTC())
}

func splitCompositeID(id string, parts int) ([]string, error) {
	values := strings.Split(id, ":")
	if len(values) != parts {
		return nil, fmt.Errorf("expected import ID with %d colon-separated parts", parts)
	}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("import ID contains an empty part")
		}
	}
	return values, nil
}

func notFound(err error) bool {
	return errors.Is(err, client.ErrNotFound)
}

func int64String(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func stringToInt64(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func mapFromTerraform(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	values := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &values, false)...)
	return values
}

func terraformMap(ctx context.Context, values map[string]string, diags *diag.Diagnostics) types.Map {
	out, d := types.MapValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return out
}
