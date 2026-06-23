package provider

import (
	"context"

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

var _ resource.Resource = (*domainResource)(nil)
var _ resource.ResourceWithConfigure = (*domainResource)(nil)
var _ resource.ResourceWithImportState = (*domainResource)(nil)

func NewDomainResource() resource.Resource { return &domainResource{} }

type domainResource struct{ client *client.Client }

type domainModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	Domain         types.String `tfsdk:"domain"`
	VerifyOnCreate types.Bool   `tfsdk:"verify_on_create"`
	Verified       types.Bool   `tfsdk:"verified"`
	SSLStatus      types.String `tfsdk:"ssl_status"`
	Verification   types.String `tfsdk:"verification"`
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
			"id":               schema.StringAttribute{Computed: true},
			"project_id":       schema.StringAttribute{Required: true, PlanModifiers: replace},
			"domain":           schema.StringAttribute{Required: true, PlanModifiers: replace},
			"verify_on_create": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}},
			"verified":         schema.BoolAttribute{Computed: true},
			"ssl_status":       schema.StringAttribute{Computed: true},
			"verification":     schema.StringAttribute{Computed: true},
		},
	}
}

func (r *domainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domain, err := r.client.CreateProjectDomain(ctx, plan.ProjectID.ValueString(), plan.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift domain failed", err.Error())
		return
	}
	if plan.VerifyOnCreate.ValueBool() {
		if err := r.client.VerifyProjectDomain(ctx, plan.ProjectID.ValueString(), domain.ID); err != nil {
			resp.Diagnostics.AddWarning("Domain verification was not queued", err.Error())
		}
	}
	plan.applyDomain(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	domains, err := r.client.ListProjectDomains(ctx, state.ProjectID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift domain failed", err.Error())
		return
	}
	for _, domain := range domains {
		if domain.ID == state.ID.ValueString() {
			state.applyDomain(&domain)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *domainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
	m.Domain = types.StringValue(d.Domain)
	m.Verified = types.BoolValue(d.Verified)
	m.SSLStatus = types.StringValue(d.SSLStatus)
	m.Verification = stringValue(d.VerificationToken)
}
