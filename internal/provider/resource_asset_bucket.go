package provider

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetBucketResource)(nil)
var _ resource.ResourceWithConfigure = (*assetBucketResource)(nil)
var _ resource.ResourceWithImportState = (*assetBucketResource)(nil)

func NewAssetBucketResource() resource.Resource { return &assetBucketResource{} }

type assetBucketResource struct{ client *client.Client }

type assetBucketModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	DefaultVisibility types.String `tfsdk:"default_visibility"`
	CacheControl      types.String `tfsdk:"cache_control"`
	Versioning        types.Bool   `tfsdk:"versioning"`
	RetentionDays     types.Int64  `tfsdk:"retention_days"`
	MaxObjectBytes    types.Int64  `tfsdk:"max_object_bytes"`
	AllowedMimeTypes  types.Set    `tfsdk:"allowed_mime_types"`
	HomeRegion        types.String `tfsdk:"home_region"`
	ReplicationPolicy types.String `tfsdk:"replication_policy"`
	CORSOrigins       types.Set    `tfsdk:"cors_origins"`
	AllowedOrigins    types.Set    `tfsdk:"allowed_origins"`
	LifecyclePolicy   types.String `tfsdk:"lifecycle_policy_json"`
	CustomDomainID    types.String `tfsdk:"custom_domain_id"`
	Revision          types.Int64  `tfsdk:"revision"`
}

func (r *assetBucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_bucket"
}

func (r *assetBucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *assetBucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a first-class StackShift Assets bucket with global replication and delivery policy.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true},
		"name": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"default_visibility":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("private")},
		"cache_control":         schema.StringAttribute{Optional: true},
		"versioning":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"retention_days":        schema.Int64Attribute{Optional: true},
		"max_object_bytes":      schema.Int64Attribute{Optional: true},
		"allowed_mime_types":    schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"home_region":           schema.StringAttribute{Optional: true, Computed: true},
		"replication_policy":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("global-3")},
		"cors_origins":          schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"allowed_origins":       schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"lifecycle_policy_json": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("{}")},
		"custom_domain_id":      schema.StringAttribute{Optional: true},
		"revision":              schema.Int64Attribute{Computed: true},
	}}
}

func (r *assetBucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetBucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := assetBucketPayload(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.CreateAssetBucket(ctx, payload)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets bucket", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, assetBucketState(ctx, bucket, &resp.Diagnostics))...)
}

func (r *assetBucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetBucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.GetAssetBucket(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets bucket", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, assetBucketState(ctx, bucket, &resp.Diagnostics))...)
}

func (r *assetBucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state assetBucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := assetBucketPayload(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.UpdateAssetBucket(ctx, state.ID.ValueString(), state.Revision.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Assets bucket", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, assetBucketState(ctx, bucket, &resp.Diagnostics))...)
}

func (r *assetBucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetBucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAssetBucket(ctx, state.ID.ValueString(), state.Revision.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("Unable to delete Assets bucket", err.Error())
	}
}

func (r *assetBucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func assetBucketPayload(ctx context.Context, model assetBucketModel, diagnostics interface{ AddError(string, string) }) map[string]any {
	mimeTypes := []string{}
	corsOrigins := []string{}
	allowedOrigins := []string{}
	if !model.AllowedMimeTypes.IsNull() && model.AllowedMimeTypes.ElementsAs(ctx, &mimeTypes, false).HasError() {
		diagnostics.AddError("Invalid allowed_mime_types", "Expected a set of strings")
	}
	if !model.CORSOrigins.IsNull() && model.CORSOrigins.ElementsAs(ctx, &corsOrigins, false).HasError() {
		diagnostics.AddError("Invalid cors_origins", "Expected a set of strings")
	}
	if !model.AllowedOrigins.IsNull() && model.AllowedOrigins.ElementsAs(ctx, &allowedOrigins, false).HasError() {
		diagnostics.AddError("Invalid allowed_origins", "Expected a set of strings")
	}
	payload := map[string]any{
		"name": model.Name.ValueString(), "default_visibility": model.DefaultVisibility.ValueString(),
		"versioning": model.Versioning.ValueBool(), "retention_days": model.RetentionDays.ValueInt64(),
		"allowed_mime_types": mimeTypes, "home_region": model.HomeRegion.ValueString(),
		"replication_policy": model.ReplicationPolicy.ValueString(), "cors_origins": corsOrigins,
		"allowed_origins": allowedOrigins, "lifecycle_policy": json.RawMessage(model.LifecyclePolicy.ValueString()),
	}
	if !model.CacheControl.IsNull() {
		payload["cache_control"] = model.CacheControl.ValueString()
	}
	if !model.MaxObjectBytes.IsNull() {
		payload["max_object_bytes"] = model.MaxObjectBytes.ValueInt64()
	}
	if !model.CustomDomainID.IsNull() {
		payload["custom_domain_id"] = model.CustomDomainID.ValueString()
	}
	return payload
}

func assetBucketState(ctx context.Context, bucket *client.AssetBucket, diagnostics interface{ AddError(string, string) }) assetBucketModel {
	mimeTypes, d1 := types.SetValueFrom(ctx, types.StringType, bucket.AllowedMimeTypes)
	corsOrigins, d2 := types.SetValueFrom(ctx, types.StringType, bucket.CORSOrigins)
	allowedOrigins, d3 := types.SetValueFrom(ctx, types.StringType, bucket.AllowedOrigins)
	if d1.HasError() || d2.HasError() || d3.HasError() {
		diagnostics.AddError("Unable to decode Assets bucket", "The API returned invalid origin or MIME sets")
	}
	state := assetBucketModel{
		ID: types.StringValue(bucket.ID), Name: types.StringValue(bucket.Name), DefaultVisibility: types.StringValue(bucket.DefaultVisibility),
		Versioning: types.BoolValue(bucket.Versioning), RetentionDays: types.Int64Value(bucket.RetentionDays),
		AllowedMimeTypes: mimeTypes, HomeRegion: types.StringValue(bucket.HomeRegion), ReplicationPolicy: types.StringValue(bucket.ReplicationPolicy),
		CORSOrigins: corsOrigins, AllowedOrigins: allowedOrigins, LifecyclePolicy: types.StringValue(string(bucket.LifecyclePolicy)),
		Revision: types.Int64Value(bucket.Revision),
	}
	if bucket.CacheControl != nil {
		state.CacheControl = types.StringValue(*bucket.CacheControl)
	} else {
		state.CacheControl = types.StringNull()
	}
	if bucket.MaxObjectBytes != nil {
		state.MaxObjectBytes = types.Int64Value(*bucket.MaxObjectBytes)
	} else {
		state.MaxObjectBytes = types.Int64Null()
	}
	if bucket.CustomDomainID != nil {
		state.CustomDomainID = types.StringValue(*bucket.CustomDomainID)
	} else {
		state.CustomDomainID = types.StringNull()
	}
	return state
}
