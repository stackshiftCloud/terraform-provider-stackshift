package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*mailDomainResource)(nil)
var _ resource.ResourceWithConfigure = (*mailDomainResource)(nil)
var _ resource.ResourceWithImportState = (*mailDomainResource)(nil)

func NewMailDomainResource() resource.Resource { return &mailDomainResource{} }

type mailDomainResource struct{ client *client.Client }

type mailDomainModel struct {
	ID                 types.String `tfsdk:"id"`
	Domain             types.String `tfsdk:"domain"`
	RegisteredDomainID types.String `tfsdk:"registered_domain_id"`
	VerifyOnCreate     types.Bool   `tfsdk:"verify_on_create"`
	Status             types.String `tfsdk:"status"`
	SPFStatus          types.String `tfsdk:"spf_status"`
	DKIMStatus         types.String `tfsdk:"dkim_status"`
	DMARCStatus        types.String `tfsdk:"dmarc_status"`
	ReturnPathStatus   types.String `tfsdk:"return_path_status"`
	ManagedDNS         types.Bool   `tfsdk:"managed_dns"`
	CreatedAt          types.String `tfsdk:"created_at"`
	VerifiedAt         types.String `tfsdk:"verified_at"`
	LastCheckedAt      types.String `tfsdk:"last_checked_at"`
}

func (r *mailDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mail_domain"
}

func (r *mailDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *mailDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a StackShift Mail sending domain.", Attributes: map[string]schema.Attribute{
		"id":                   schema.StringAttribute{Computed: true},
		"domain":               schema.StringAttribute{Optional: true, PlanModifiers: replace, MarkdownDescription: "Sending domain. Set exactly one of domain or registered_domain_id."},
		"registered_domain_id": schema.StringAttribute{Optional: true, PlanModifiers: replace, MarkdownDescription: "Existing StackShift-managed domain ID. Set exactly one of registered_domain_id or domain."},
		"verify_on_create":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}},
		"status":               schema.StringAttribute{Computed: true},
		"spf_status":           schema.StringAttribute{Computed: true},
		"dkim_status":          schema.StringAttribute{Computed: true},
		"dmarc_status":         schema.StringAttribute{Computed: true},
		"return_path_status":   schema.StringAttribute{Computed: true},
		"managed_dns":          schema.BoolAttribute{Computed: true},
		"created_at":           schema.StringAttribute{Computed: true},
		"verified_at":          schema.StringAttribute{Computed: true},
		"last_checked_at":      schema.StringAttribute{Computed: true},
	}}
}

func (r *mailDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mailDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	domainName := strings.TrimSpace(plan.Domain.ValueString())
	registeredDomainID := strings.TrimSpace(plan.RegisteredDomainID.ValueString())
	if resp.Diagnostics.HasError() || (domainName == "") == (registeredDomainID == "") {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Invalid Mail domain", "Set exactly one of domain or registered_domain_id.")
		}
		return
	}
	item, err := r.client.CreateMailDomain(ctx, domainName, registeredDomainID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Mail domain", err.Error())
		return
	}
	if plan.VerifyOnCreate.ValueBool() {
		item, err = r.client.VerifyMailDomain(ctx, item.ID)
		if err != nil {
			resp.Diagnostics.AddWarning("Mail domain verification did not complete", err.Error())
		}
	}
	plan.apply(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mailDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mailDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.GetMailDomain(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Mail domain", err.Error())
		return
	}
	state.apply(item)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mailDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mailDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mailDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mailDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteMailDomain(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete Mail domain", err.Error())
		}
	}
}

func (r *mailDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *mailDomainModel) apply(item *client.MailDomain) {
	m.ID = types.StringValue(item.ID)
	m.Domain = types.StringValue(item.Domain)
	m.Status = types.StringValue(item.Status)
	m.SPFStatus = types.StringValue(item.SPFStatus)
	m.DKIMStatus = types.StringValue(item.DKIMStatus)
	m.DMARCStatus = types.StringValue(item.DMARCStatus)
	m.ReturnPathStatus = types.StringValue(item.ReturnPathStatus)
	m.ManagedDNS = types.BoolValue(item.ManagedDNS)
	m.CreatedAt = timeString(item.CreatedAt)
	m.VerifiedAt = optionalTimeString(item.VerifiedAt)
	m.LastCheckedAt = optionalTimeString(item.LastCheckedAt)
}
