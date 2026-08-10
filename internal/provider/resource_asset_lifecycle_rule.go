package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetLifecycleRuleResource)(nil)
var _ resource.ResourceWithConfigure = (*assetLifecycleRuleResource)(nil)
var _ resource.ResourceWithImportState = (*assetLifecycleRuleResource)(nil)

func NewAssetLifecycleRuleResource() resource.Resource { return &assetLifecycleRuleResource{} }

type assetLifecycleRuleResource struct{ client *client.Client }

type assetLifecycleRuleModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Prefix    types.String `tfsdk:"prefix"`
	Action    types.String `tfsdk:"action"`
	AgeDays   types.Int64  `tfsdk:"age_days"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
	LastRunAt types.String `tfsdk:"last_run_at"`
	LastError types.String `tfsdk:"last_error"`
}

func (r *assetLifecycleRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_lifecycle_rule"
}

func (r *assetLifecycleRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *assetLifecycleRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	stringReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{MarkdownDescription: "Manages an immutable StackShift Assets lifecycle rule.", Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"name":       schema.StringAttribute{Required: true, PlanModifiers: stringReplace},
		"prefix":     schema.StringAttribute{Optional: true, PlanModifiers: stringReplace},
		"action":     schema.StringAttribute{Required: true, MarkdownDescription: "Either `delete` or `expire_versions`.", PlanModifiers: stringReplace},
		"age_days":   schema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"enabled":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}},
		"created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true},
		"last_run_at": schema.StringAttribute{Computed: true}, "last_error": schema.StringAttribute{Computed: true},
	}}
}

func (r *assetLifecycleRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetLifecycleRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.CreateAssetLifecycleRule(ctx, map[string]any{"name": plan.Name.ValueString(), "prefix": plan.Prefix.ValueString(), "action": plan.Action.ValueString(), "age_days": plan.AgeDays.ValueInt64(), "enabled": plan.Enabled.ValueBool()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets lifecycle rule", err.Error())
		return
	}
	plan.apply(rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetLifecycleRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetLifecycleRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetAssetLifecycleRule(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets lifecycle rule", err.Error())
		return
	}
	state.apply(rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetLifecycleRuleResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *assetLifecycleRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetLifecycleRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteAssetLifecycleRule(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete Assets lifecycle rule", err.Error())
		}
	}
}

func (r *assetLifecycleRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *assetLifecycleRuleModel) apply(rule *client.AssetLifecycleRule) {
	m.ID = types.StringValue(rule.ID)
	m.Name = types.StringValue(rule.Name)
	m.Prefix = types.StringValue(rule.Prefix)
	m.Action = types.StringValue(rule.Action)
	m.AgeDays = types.Int64Value(rule.AgeDays)
	m.Enabled = types.BoolValue(rule.Enabled)
	m.CreatedAt = types.StringValue(rule.CreatedAt.UTC().Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(rule.UpdatedAt.UTC().Format(time.RFC3339))
	m.LastRunAt = optionalAssetTime(rule.LastRunAt)
	m.LastError = optionalAssetString(rule.LastError)
}

func optionalAssetTime(value *time.Time) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.UTC().Format(time.RFC3339))
}
func optionalAssetString(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}
