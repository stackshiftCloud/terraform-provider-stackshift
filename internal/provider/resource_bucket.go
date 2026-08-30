package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*bucketResource)(nil)
var _ resource.ResourceWithConfigure = (*bucketResource)(nil)
var _ resource.ResourceWithImportState = (*bucketResource)(nil)

func NewBucketResource() resource.Resource { return &bucketResource{} }

type bucketResource struct{ client *client.Client }

type bucketModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Region          types.String `tfsdk:"region"`
	Visibility      types.String `tfsdk:"visibility"`
	ProjectID       types.String `tfsdk:"project_id"`
	AccessKeyLabel  types.String `tfsdk:"access_key_label"`
	ForceDestroy    types.Bool   `tfsdk:"force_destroy"`
	Versioning      types.Bool   `tfsdk:"versioning_enabled"`
	QuotaBytes      types.Int64  `tfsdk:"quota_bytes"`
	RetentionDays   types.Int64  `tfsdk:"default_retention_days"`
	TierAfterDays   types.Int64  `tfsdk:"tier_after_days"`
	WebsiteEnabled  types.Bool   `tfsdk:"website_enabled"`
	WebsiteIndex    types.String `tfsdk:"website_index"`
	WebsiteError    types.String `tfsdk:"website_error"`
	EncryptionMode  types.String `tfsdk:"encryption_mode"`
	KMSKeyID        types.String `tfsdk:"kms_key_id"`
	CustomDomainID  types.String `tfsdk:"custom_domain_id"`
	Endpoint        types.String `tfsdk:"endpoint"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	ObjectCount     types.Int64  `tfsdk:"object_count"`
	SizeBytes       types.Int64  `tfsdk:"size_bytes"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

func (r *bucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *bucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *bucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StackShift S2 S3-compatible object-storage bucket. The generated secret access key is returned once and stored in sensitive Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true},
			"name":             schema.StringAttribute{Required: true, PlanModifiers: replace},
			"region":           schema.StringAttribute{Required: true, PlanModifiers: replace},
			"visibility":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("private"), PlanModifiers: replace},
			"project_id":       schema.StringAttribute{Optional: true, PlanModifiers: replace},
			"access_key_label": schema.StringAttribute{Optional: true, PlanModifiers: replace},
			"force_destroy":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"versioning_enabled": schema.BoolAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"quota_bytes": schema.Int64Attribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"default_retention_days": schema.Int64Attribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"tier_after_days": schema.Int64Attribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"website_enabled": schema.BoolAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"website_index": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"website_error": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"encryption_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Object encryption mode: `none`, `sse-s2`, or platform-managed `sse-kms`.",
			},
			"kms_key_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "StackShift-managed KMS key ARN when `encryption_mode` is `sse-kms`.",
			},
			"custom_domain_id": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint":          schema.StringAttribute{Computed: true},
			"access_key_id":     schema.StringAttribute{Computed: true},
			"secret_access_key": schema.StringAttribute{Computed: true, Sensitive: true},
			"object_count":      schema.Int64Attribute{Computed: true},
			"size_bytes":        schema.Int64Attribute{Computed: true},
			"created_at":        schema.StringAttribute{Computed: true},
			"updated_at":        schema.StringAttribute{Computed: true},
		},
	}
}

func (r *bucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	settings := plan.settingsRequest()
	bucket, credentials, err := r.client.CreateBucket(ctx, client.CreateBucketRequest{
		Name:       plan.Name.ValueString(),
		Region:     plan.Region.ValueString(),
		Visibility: plan.Visibility.ValueString(),
		ProjectID:  stringPtr(plan.ProjectID),
		Label:      plan.AccessKeyLabel.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift S2 bucket failed", err.Error())
		return
	}
	plan.applyBucket(bucket)
	plan.AccessKeyID = types.StringValue(credentials.AccessKeyID)
	plan.SecretAccessKey = types.StringValue(credentials.SecretAccessKey)
	if plan.Endpoint.ValueString() == "" {
		plan.Endpoint = types.StringValue(credentials.Endpoint)
	}
	updated, err := r.client.UpdateBucketSettings(
		ctx,
		bucket.ID,
		settings,
	)
	if err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Configure StackShift S2 bucket failed", err.Error())
		return
	}
	plan.applyBucket(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, err := r.client.GetBucket(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift S2 bucket failed", err.Error())
		return
	}
	state.applyBucket(bucket)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan bucketModel
	var state bucketModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateBucketSettings(
		ctx,
		state.ID.ValueString(),
		plan.settingsRequest(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Update StackShift S2 bucket failed", err.Error())
		return
	}
	plan.applyBucket(updated)
	plan.AccessKeyID = state.AccessKeyID
	plan.SecretAccessKey = state.SecretAccessKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bucketModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBucket(ctx, state.ID.ValueString(), state.ForceDestroy.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift S2 bucket failed", err.Error())
	}
}

func (r *bucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *bucketModel) applyBucket(bucket *client.Bucket) {
	m.ID = types.StringValue(bucket.ID)
	m.Name = types.StringValue(bucket.Name)
	m.Region = types.StringValue(bucket.Region)
	m.Visibility = types.StringValue(bucket.Visibility)
	m.ProjectID = stringValue(bucket.ProjectID)
	m.Versioning = types.BoolValue(bucket.VersioningEnabled)
	m.QuotaBytes = int64Value(bucket.QuotaBytes)
	m.RetentionDays = types.Int64Value(bucket.DefaultRetentionDays)
	m.TierAfterDays = types.Int64Value(bucket.TierAfterDays)
	m.WebsiteEnabled = types.BoolValue(bucket.WebsiteEnabled)
	m.WebsiteIndex = types.StringValue(bucket.WebsiteIndex)
	m.WebsiteError = types.StringValue(bucket.WebsiteError)
	m.EncryptionMode = types.StringValue(bucket.EncryptionMode)
	if bucket.KMSKeyID == "" {
		m.KMSKeyID = types.StringNull()
	} else {
		m.KMSKeyID = types.StringValue(bucket.KMSKeyID)
	}
	m.CustomDomainID = stringValue(bucket.CustomDomainID)
	m.Endpoint = types.StringValue(bucket.Endpoint)
	m.ObjectCount = types.Int64Value(bucket.ObjectCount)
	m.SizeBytes = types.Int64Value(bucket.SizeBytes)
	m.CreatedAt = timeString(bucket.CreatedAt)
	m.UpdatedAt = timeString(bucket.UpdatedAt)
}

func (m *bucketModel) settingsRequest() client.UpdateBucketSettingsRequest {
	websiteIndex := m.WebsiteIndex.ValueString()
	if m.WebsiteIndex.IsNull() || m.WebsiteIndex.IsUnknown() || websiteIndex == "" {
		websiteIndex = "index.html"
	}
	encryptionMode := m.EncryptionMode.ValueString()
	if m.EncryptionMode.IsNull() || m.EncryptionMode.IsUnknown() || encryptionMode == "" {
		encryptionMode = "none"
	}
	return client.UpdateBucketSettingsRequest{
		VersioningEnabled:    m.Versioning.ValueBool(),
		QuotaBytes:           int64Ptr(m.QuotaBytes),
		DefaultRetentionDays: m.RetentionDays.ValueInt64(),
		TierAfterDays:        m.TierAfterDays.ValueInt64(),
		WebsiteEnabled:       m.WebsiteEnabled.ValueBool(),
		WebsiteIndex:         websiteIndex,
		WebsiteError:         m.WebsiteError.ValueString(),
		EncryptionMode:       encryptionMode,
		CustomDomainID:       stringPtr(m.CustomDomainID),
	}
}
