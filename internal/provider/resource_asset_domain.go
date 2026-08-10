package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetDomainResource)(nil)
var _ resource.ResourceWithConfigure = (*assetDomainResource)(nil)
var _ resource.ResourceWithImportState = (*assetDomainResource)(nil)

func NewAssetDomainResource() resource.Resource { return &assetDomainResource{} }

type assetDomainResource struct{ client *client.Client }

type assetDomainModel struct {
	ID                types.String `tfsdk:"id"`
	Domain            types.String `tfsdk:"domain"`
	Verify            types.Bool   `tfsdk:"verify"`
	Status            types.String `tfsdk:"status"`
	VerificationName  types.String `tfsdk:"verification_name"`
	VerificationValue types.String `tfsdk:"verification_value"`
	LastError         types.String `tfsdk:"last_error"`
	ProviderDomainID  types.String `tfsdk:"provider_domain_id"`
	CertificateID     types.String `tfsdk:"certificate_id"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	VerifiedAt        types.String `tfsdk:"verified_at"`
}

func (r *assetDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_domain"
}
func (r *assetDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *assetDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a verified custom delivery domain for StackShift Assets.", Attributes: map[string]schema.Attribute{
		"id":     schema.StringAttribute{Computed: true},
		"domain": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"verify": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Set true after publishing the returned verification DNS record."},
		"status": schema.StringAttribute{Computed: true}, "verification_name": schema.StringAttribute{Computed: true}, "verification_value": schema.StringAttribute{Computed: true},
		"last_error": schema.StringAttribute{Computed: true}, "provider_domain_id": schema.StringAttribute{Computed: true}, "certificate_id": schema.StringAttribute{Computed: true},
		"created_at": schema.StringAttribute{Computed: true}, "updated_at": schema.StringAttribute{Computed: true}, "verified_at": schema.StringAttribute{Computed: true},
	}}
}

func (r *assetDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.CreateAssetDomain(ctx, plan.Domain.ValueString())
	if err == nil && plan.Verify.ValueBool() {
		domain, err = r.client.VerifyAssetDomain(ctx, domain.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets domain", err.Error())
		return
	}
	plan.apply(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.GetAssetDomain(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets domain", err.Error())
		return
	}
	state.apply(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state assetDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.GetAssetDomain(ctx, state.ID.ValueString())
	if err == nil && plan.Verify.ValueBool() && !state.Verify.ValueBool() {
		domain, err = r.client.VerifyAssetDomain(ctx, state.ID.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify Assets domain", err.Error())
		return
	}
	plan.apply(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteAssetDomain(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete Assets domain", err.Error())
		}
	}
}

func (r *assetDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *assetDomainModel) apply(domain *client.AssetCustomDomain) {
	m.ID = types.StringValue(domain.ID)
	m.Domain = types.StringValue(domain.Domain)
	m.Status = types.StringValue(domain.Status)
	m.VerificationName = types.StringValue(domain.VerificationName)
	m.VerificationValue = types.StringValue(domain.VerificationValue)
	m.LastError = optionalAssetString(domain.LastError)
	m.ProviderDomainID = optionalAssetString(domain.ProviderDomainID)
	m.CertificateID = optionalAssetString(domain.CertificateID)
	m.CreatedAt = types.StringValue(domain.CreatedAt.UTC().Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(domain.UpdatedAt.UTC().Format(time.RFC3339))
	m.VerifiedAt = optionalAssetTime(domain.VerifiedAt)
}
