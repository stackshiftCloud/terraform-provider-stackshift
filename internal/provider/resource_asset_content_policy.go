package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetContentPolicyResource)(nil)
var _ resource.ResourceWithConfigure = (*assetContentPolicyResource)(nil)
var _ resource.ResourceWithImportState = (*assetContentPolicyResource)(nil)

func NewAssetContentPolicyResource() resource.Resource { return &assetContentPolicyResource{} }

type assetContentPolicyResource struct{ client *client.Client }

type assetContentPolicyModel struct {
	ID               types.String `tfsdk:"id"`
	AllowedMimeTypes types.Set    `tfsdk:"allowed_mime_types"`
	MaxImageBytes    types.Int64  `tfsdk:"max_image_bytes"`
	MaxVideoBytes    types.Int64  `tfsdk:"max_video_bytes"`
	MaxOtherBytes    types.Int64  `tfsdk:"max_other_bytes"`
	RequireScan      types.Bool   `tfsdk:"require_scan"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func (r *assetContentPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_content_policy"
}

func (r *assetContentPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *assetContentPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages the account-wide StackShift Assets content admission and malware-scan policy.", Attributes: map[string]schema.Attribute{
		"id":                 schema.StringAttribute{Computed: true},
		"allowed_mime_types": schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"max_image_bytes":    schema.Int64Attribute{Required: true},
		"max_video_bytes":    schema.Int64Attribute{Required: true},
		"max_other_bytes":    schema.Int64Attribute{Required: true},
		"require_scan":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"created_at":         schema.StringAttribute{Computed: true},
		"updated_at":         schema.StringAttribute{Computed: true},
	}}
}

func (r *assetContentPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetContentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := assetContentPolicyPayload(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.UpdateAssetContentPolicy(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets content policy", err.Error())
		return
	}
	state := assetContentPolicyState(ctx, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetContentPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	policy, err := r.client.GetAssetContentPolicy(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets content policy", err.Error())
		return
	}
	state := assetContentPolicyState(ctx, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetContentPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan assetContentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := assetContentPolicyPayload(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.UpdateAssetContentPolicy(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Assets content policy", err.Error())
		return
	}
	state := assetContentPolicyState(ctx, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetContentPolicyResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	_, err := r.client.UpdateAssetContentPolicy(ctx, map[string]any{
		"allowed_mime_types": []string{}, "max_image_bytes": 0, "max_video_bytes": 0,
		"max_other_bytes": 0, "require_scan": true,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to reset Assets content policy", err.Error())
	}
}

func (r *assetContentPolicyResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "default")...)
}

func assetContentPolicyPayload(ctx context.Context, model assetContentPolicyModel, diagnostics interface{ AddError(string, string) }) map[string]any {
	allowed := []string{}
	if !model.AllowedMimeTypes.IsNull() && model.AllowedMimeTypes.ElementsAs(ctx, &allowed, false).HasError() {
		diagnostics.AddError("Invalid allowed_mime_types", "Expected a set of MIME type strings")
	}
	return map[string]any{
		"allowed_mime_types": allowed, "max_image_bytes": model.MaxImageBytes.ValueInt64(),
		"max_video_bytes": model.MaxVideoBytes.ValueInt64(), "max_other_bytes": model.MaxOtherBytes.ValueInt64(),
		"require_scan": model.RequireScan.ValueBool(),
	}
}

func assetContentPolicyState(ctx context.Context, policy *client.AssetContentPolicy, diagnostics interface{ AddError(string, string) }) assetContentPolicyModel {
	allowed, result := types.SetValueFrom(ctx, types.StringType, policy.AllowedMimeTypes)
	if result.HasError() {
		diagnostics.AddError("Unable to decode Assets content policy", "The API returned invalid MIME types")
	}
	state := assetContentPolicyModel{
		ID: types.StringValue("default"), AllowedMimeTypes: allowed,
		MaxImageBytes: types.Int64Value(policy.MaxImageBytes), MaxVideoBytes: types.Int64Value(policy.MaxVideoBytes),
		MaxOtherBytes: types.Int64Value(policy.MaxOtherBytes), RequireScan: types.BoolValue(policy.RequireScan),
	}
	state.CreatedAt, state.UpdatedAt = types.StringNull(), types.StringNull()
	if !policy.CreatedAt.IsZero() {
		state.CreatedAt = types.StringValue(policy.CreatedAt.UTC().Format(time.RFC3339))
	}
	if !policy.UpdatedAt.IsZero() {
		state.UpdatedAt = types.StringValue(policy.UpdatedAt.UTC().Format(time.RFC3339))
	}
	return state
}
