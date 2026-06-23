package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

// bucketResource is the S3-compatible object-store bucket resource. Its schema
// is defined and locked, but it is intentionally NOT registered in provider.go:
// the StackShift object-store API does not exist yet, so the resource must not
// claim CRUD support for endpoints that are not implemented. Register it in
// provider.Resources() and replace the CRUD bodies once the bucket API ships.

const bucketUnavailableSummary = "StackShift object storage is not available"
const bucketUnavailableDetail = "The stackshift_bucket resource is not enabled: the S3-compatible object-store API has not shipped yet."

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
	Endpoint        types.String `tfsdk:"endpoint"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
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
		MarkdownDescription: "Manages an S3-compatible storage bucket. Not yet available — pending the StackShift object-store API.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true},
			"name":              schema.StringAttribute{Required: true, PlanModifiers: replace},
			"region":            schema.StringAttribute{Required: true, PlanModifiers: replace},
			"visibility":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("private")},
			"project_id":        schema.StringAttribute{Optional: true, PlanModifiers: replace},
			"endpoint":          schema.StringAttribute{Computed: true},
			"access_key_id":     schema.StringAttribute{Computed: true, Sensitive: true},
			"secret_access_key": schema.StringAttribute{Computed: true, Sensitive: true},
		},
	}
}

func (r *bucketResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(bucketUnavailableSummary, bucketUnavailableDetail)
}

func (r *bucketResource) Read(_ context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.AddError(bucketUnavailableSummary, bucketUnavailableDetail)
}

func (r *bucketResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(bucketUnavailableSummary, bucketUnavailableDetail)
}

func (r *bucketResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError(bucketUnavailableSummary, bucketUnavailableDetail)
}

func (r *bucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
