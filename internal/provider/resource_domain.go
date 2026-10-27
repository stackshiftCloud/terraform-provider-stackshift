package provider

import (
	"context"

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

var _ resource.Resource = (*domainResource)(nil)
var _ resource.ResourceWithConfigure = (*domainResource)(nil)
var _ resource.ResourceWithImportState = (*domainResource)(nil)

func NewDomainResource() resource.Resource { return &domainResource{} }

type domainResource struct{ client *client.Client }

type domainModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Domain               types.String `tfsdk:"domain"`
	Primary              types.Bool   `tfsdk:"primary"`
	IsPrimary            types.Bool   `tfsdk:"is_primary"`
	VerifyOnCreate       types.Bool   `tfsdk:"verify_on_create"`
	Verified             types.Bool   `tfsdk:"verified"`
	SSLStatus            types.String `tfsdk:"ssl_status"`
	VerificationStatus   types.String `tfsdk:"verification_status"`
	VerificationTXTName  types.String `tfsdk:"verification_txt_name"`
	VerificationTXTValue types.String `tfsdk:"verification_txt_value"`
	DNSWarning           types.String `tfsdk:"dns_warning"`
}

func (r *domainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *domainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *domainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
			"domain":     schema.StringAttribute{Required: true, PlanModifiers: replace},
			"primary": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				MarkdownDescription: "Make this the project's primary domain. A domain becomes primary only after it is verified; `is_primary` shows the current state.",
			},
			"is_primary":             schema.BoolAttribute{Computed: true},
			"verify_on_create":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"verified":               schema.BoolAttribute{Computed: true},
			"ssl_status":             schema.StringAttribute{Computed: true},
			"verification_status":    schema.StringAttribute{Computed: true},
			"verification_txt_name":  schema.StringAttribute{Computed: true, MarkdownDescription: "TXT record name that proves ownership while the domain is unverified."},
			"verification_txt_value": schema.StringAttribute{Computed: true, MarkdownDescription: "TXT record value that proves ownership while the domain is unverified."},
			"dns_warning":            schema.StringAttribute{Computed: true, MarkdownDescription: "Set when the hostname's DNS points at another project's address, with the record to fix."},
		},
	}
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := plan.ProjectID.ValueString()
	domain, err := r.client.CreateProjectDomain(ctx, projectID, plan.Domain.ValueString(), plan.Primary.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift domain failed", err.Error())
		return
	}
	if plan.VerifyOnCreate.ValueBool() && domain.VerificationStatus != "verified" {
		if verified, err := r.client.VerifyProjectDomain(ctx, projectID, domain.ID); err != nil {
			resp.Diagnostics.AddWarning("Domain is not verified yet", err.Error())
		} else {
			domain = verified
		}
	}
	plan.ID = types.StringValue(domain.ID)
	domain = r.ensurePrimary(ctx, plan, domain, &resp.Diagnostics)
	plan.applyDomain(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.findDomain(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if notFound(err) || err == nil && domain == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift domain failed", err.Error())
		return
	}
	state.applyDomain(domain)
	state.Primary = configuredPrimary(state.Primary, domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.findDomain(ctx, plan.ProjectID.ValueString(), plan.ID.ValueString())
	if err == nil && domain == nil {
		err = client.ErrNotFound
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift domain failed", err.Error())
		return
	}
	domain = r.ensurePrimary(ctx, plan, domain, &resp.Diagnostics)
	plan.applyDomain(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) ensurePrimary(ctx context.Context, plan domainModel, domain *client.ProjectDomain, diags *diag.Diagnostics) *client.ProjectDomain {
	if !plan.Primary.ValueBool() || domain.IsPrimary {
		return domain
	}
	if domain.VerificationStatus != "verified" {
		diags.AddWarning("Domain is not primary yet", domain.Hostname+" becomes primary on the next apply after it is verified.")
		return domain
	}
	projectID := plan.ProjectID.ValueString()
	if err := r.client.SetProjectDomainPrimary(ctx, projectID, domain.ID); err != nil {
		diags.AddError("Set primary StackShift domain failed", err.Error())
		return domain
	}
	refreshed, err := r.findDomain(ctx, projectID, domain.ID)
	if err != nil || refreshed == nil {
		domain.IsPrimary = true
		return domain
	}
	return refreshed
}

func (r *domainResource) findDomain(ctx context.Context, projectID, id string) (*client.ProjectDomain, error) {
	domains, err := r.client.ListProjectDomains(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		if domain.ID == id {
			return &domain, nil
		}
	}
	return nil, nil
}

func configuredPrimary(configured types.Bool, domain *client.ProjectDomain) types.Bool {
	if configured.IsNull() || configured.IsUnknown() {
		return types.BoolValue(domain.IsPrimary)
	}
	if configured.ValueBool() && !domain.IsPrimary && domain.VerificationStatus == "verified" {
		return types.BoolValue(false)
	}
	return configured
}

func (r *domainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProjectDomain(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift domain failed", err.Error())
	}
}

func (r *domainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *domainModel) applyDomain(d *client.ProjectDomain) {
	m.ID = types.StringValue(d.ID)
	if d.ProjectID != "" {
		m.ProjectID = types.StringValue(d.ProjectID)
	}
	m.Domain = types.StringValue(d.Hostname)
	m.IsPrimary = types.BoolValue(d.IsPrimary)
	m.Verified = types.BoolValue(d.VerificationStatus == "verified")
	m.SSLStatus = types.StringValue(d.SSLStatus)
	m.VerificationStatus = types.StringValue(d.VerificationStatus)
	m.VerificationTXTName = types.StringValue("")
	m.VerificationTXTValue = types.StringValue("")
	if d.VerificationRecord != nil {
		m.VerificationTXTName = types.StringValue(d.VerificationRecord.Name)
		m.VerificationTXTValue = types.StringValue(d.VerificationRecord.Value)
	}
	m.DNSWarning = types.StringValue("")
	if d.DNSWarning != nil {
		m.DNSWarning = types.StringValue(d.DNSWarning.Message)
	}
}
