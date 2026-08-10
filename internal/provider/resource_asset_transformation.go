package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetTransformationResource)(nil)
var _ resource.ResourceWithConfigure = (*assetTransformationResource)(nil)
var _ resource.ResourceWithImportState = (*assetTransformationResource)(nil)

func NewAssetTransformationResource() resource.Resource { return &assetTransformationResource{} }

type assetTransformationResource struct{ client *client.Client }

type assetTransformationModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Spec           types.String `tfsdk:"spec"`
	NormalizedSpec types.String `tfsdk:"normalized_spec"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *assetTransformationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_transformation"
}

func (r *assetTransformationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *assetTransformationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a named, deterministic StackShift Assets transformation preset.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true},
		"name":            schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"spec":            schema.StringAttribute{Required: true},
		"normalized_spec": schema.StringAttribute{Computed: true},
		"created_at":      schema.StringAttribute{Computed: true},
		"updated_at":      schema.StringAttribute{Computed: true},
	}}
}

func (r *assetTransformationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetTransformationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.UpsertAssetTransformation(ctx, map[string]any{"name": plan.Name.ValueString(), "spec": plan.Spec.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets transformation", err.Error())
		return
	}
	plan.apply(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetTransformationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetTransformationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.GetAssetTransformation(ctx, state.Name.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets transformation", err.Error())
		return
	}
	state.apply(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetTransformationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan assetTransformationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.UpsertAssetTransformation(ctx, map[string]any{"name": plan.Name.ValueString(), "spec": plan.Spec.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Assets transformation", err.Error())
		return
	}
	plan.apply(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetTransformationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetTransformationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteAssetTransformation(ctx, state.Name.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete Assets transformation", err.Error())
		}
	}
}

func (r *assetTransformationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (m *assetTransformationModel) apply(item *client.AssetTransformation) {
	m.ID = types.StringValue(item.ID)
	m.Name = types.StringValue(item.Name)
	m.Spec = types.StringValue(item.Spec)
	m.NormalizedSpec = types.StringValue(item.NormalizedSpec)
	m.CreatedAt = types.StringValue(item.CreatedAt.UTC().Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(item.UpdatedAt.UTC().Format(time.RFC3339))
}
