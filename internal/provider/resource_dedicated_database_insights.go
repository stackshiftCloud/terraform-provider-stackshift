package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseInsightsResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseInsightsResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dedicatedDatabaseInsightsResource)(nil)

var insightsRetentionOptions = []int64{7, 30, 90}

func NewDedicatedDatabaseInsightsResource() resource.Resource {
	return &dedicatedDatabaseInsightsResource{}
}

type dedicatedDatabaseInsightsResource struct{ client *client.Client }

type dedicatedDatabaseInsightsModel struct {
	ID                 types.String `tfsdk:"id"`
	DatabaseID         types.String `tfsdk:"database_id"`
	RetentionDays      types.Int64  `tfsdk:"retention_days"`
	AcceptPriceChanges types.Bool   `tfsdk:"accept_price_changes"`
	IncludedDays       types.Int64  `tfsdk:"included_days"`
}

func (r *dedicatedDatabaseInsightsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_insights"
}

func (r *dedicatedDatabaseInsightsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedDatabaseInsightsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "How long a Dedicated database keeps its performance history. Keeping history longer than the included days is a paid change. Destroying this resource returns the database to the included days.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"database_id":    schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"retention_days": schema.Int64Attribute{Required: true, Description: "7, 30 or 90. Changes in place."},
			"accept_price_changes": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Allow a change that costs extra to be charged to the wallet. When false, a paid change stops with the quote."},
			"included_days": schema.Int64Attribute{Computed: true, Description: "How many days of history come with the database at no extra cost."},
		},
	}
}

func (r *dedicatedDatabaseInsightsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var days types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("retention_days"), &days)...)
	if days.IsNull() || days.IsUnknown() {
		return
	}
	for _, option := range insightsRetentionOptions {
		if days.ValueInt64() == option {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(path.Root("retention_days"), "Unsupported value", fmt.Sprintf("Keep performance history for one of %v days.", insightsRetentionOptions))
}

func (r *dedicatedDatabaseInsightsResource) apply(ctx context.Context, plan *dedicatedDatabaseInsightsModel, days int64, diags *diag.Diagnostics) bool {
	databaseID := plan.DatabaseID.ValueString()
	if !paidChange(diags, "performance history change", plan.AcceptPriceChanges.ValueBool(), func(confirm *int64) error {
		return r.client.SetDedicatedInsights(ctx, databaseID, days, confirm)
	}) {
		return false
	}
	insights, err := r.client.GetDedicatedInsights(ctx, databaseID)
	if err != nil {
		diags.AddError("Read StackShift performance history settings failed", dedicatedErrorDetail(err))
		return false
	}
	plan.ID, plan.RetentionDays, plan.IncludedDays = plan.DatabaseID, types.Int64Value(insights.RetentionDays), types.Int64Value(insights.IncludedDays)
	return true
}

func (r *dedicatedDatabaseInsightsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseInsightsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !r.apply(ctx, &plan, plan.RetentionDays.ValueInt64(), &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseInsightsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseInsightsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	insights, err := r.client.GetDedicatedInsights(ctx, state.DatabaseID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift performance history settings failed", dedicatedErrorDetail(err))
		return
	}
	state.RetentionDays, state.IncludedDays = types.Int64Value(insights.RetentionDays), types.Int64Value(insights.IncludedDays)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseInsightsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dedicatedDatabaseInsightsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() || !r.apply(ctx, &plan, plan.RetentionDays.ValueInt64(), &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseInsightsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabaseInsightsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || state.RetentionDays.Equal(state.IncludedDays) {
		return
	}
	paidChange(&resp.Diagnostics, "performance history change", state.AcceptPriceChanges.ValueBool(), func(confirm *int64) error {
		err := r.client.SetDedicatedInsights(ctx, state.DatabaseID.ValueString(), state.IncludedDays.ValueInt64(), confirm)
		if notFound(err) {
			return nil
		}
		return err
	})
}

func (r *dedicatedDatabaseInsightsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("accept_price_changes"), false)...)
}
