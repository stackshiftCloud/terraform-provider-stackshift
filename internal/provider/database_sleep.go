package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

const (
	sleepMinMinutes = 5
	sleepMaxMinutes = 1440
)

func sleepAfterMinutesAttribute(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Description: description}
}

func validateSleepAfterMinutes(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) types.Int64 {
	var minutes types.Int64
	diags.Append(config.GetAttribute(ctx, path.Root("sleep_after_minutes"), &minutes)...)
	if minutes.IsNull() || minutes.IsUnknown() {
		return minutes
	}
	if value := minutes.ValueInt64(); value != 0 && (value < sleepMinMinutes || value > sleepMaxMinutes) {
		diags.AddAttributeError(path.Root("sleep_after_minutes"), "Unsupported value",
			fmt.Sprintf("Choose %d to %d minutes of quiet before the database sleeps, or 0 to keep it awake.", sleepMinMinutes, sleepMaxMinutes))
	}
	return minutes
}

func sleepOnlyForPostgres(diags *diag.Diagnostics, minutes types.Int64) {
	if !minutes.IsNull() && !minutes.IsUnknown() && minutes.ValueInt64() > 0 {
		diags.AddAttributeError(path.Root("sleep_after_minutes"), "Sleep when idle is only for PostgreSQL",
			"Only PostgreSQL databases can sleep when idle. Remove sleep_after_minutes or use a PostgreSQL database.")
	}
}

func sleepAfterMinutes(prior types.Int64, sleep *client.DatabaseSleep) types.Int64 {
	if sleep != nil && sleep.Enabled {
		return types.Int64Value(sleep.AfterMinutes)
	}
	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueInt64() == 0 {
		return prior
	}
	return types.Int64Null()
}

func sleepNotAppliedWarning(diags *diag.Diagnostics, id string, err error) {
	diags.AddWarning("Sleep when idle wasn't turned on",
		fmt.Sprintf("The database %s was created, but sleep_after_minutes didn't apply: %s\n\nThe database is kept in Terraform state. The next plan shows the difference, and the next apply tries again.", id, err))
}
