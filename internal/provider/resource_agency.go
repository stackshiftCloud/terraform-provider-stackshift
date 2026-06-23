package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type agencyClientResource struct{ client *client.Client }
type agencyResourceAssignmentResource struct{ client *client.Client }

type agencyClientModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Company         types.String `tfsdk:"company"`
	Email           types.String `tfsdk:"email"`
	Phone           types.String `tfsdk:"phone"`
	Status          types.String `tfsdk:"status"`
	BillingCurrency types.String `tfsdk:"billing_currency"`
	Notes           types.String `tfsdk:"notes"`
	TeamID          types.String `tfsdk:"team_id"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

type agencyResourceAssignmentModel struct {
	ID           types.String `tfsdk:"id"`
	ClientID     types.String `tfsdk:"client_id"`
	ResourceType types.String `tfsdk:"resource_type"`
	ResourceID   types.String `tfsdk:"resource_id"`
	Label        types.String `tfsdk:"label"`
	TeamID       types.String `tfsdk:"team_id"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func NewAgencyClientResource() resource.Resource { return &agencyClientResource{} }
func NewAgencyResourceAssignmentResource() resource.Resource {
	return &agencyResourceAssignmentResource{}
}

func (r *agencyClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agency_client"
}
func (r *agencyClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *agencyClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Required: true},
		"company": schema.StringAttribute{Optional: true}, "email": schema.StringAttribute{Optional: true},
		"phone": schema.StringAttribute{Optional: true}, "status": schema.StringAttribute{Optional: true, Computed: true},
		"billing_currency": schema.StringAttribute{Optional: true, Computed: true}, "notes": schema.StringAttribute{Optional: true},
		"team_id": schema.StringAttribute{Optional: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *agencyClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agencyClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.CreateAgencyClient(ctx, plan.request())
	if err != nil {
		resp.Diagnostics.AddError("Create agency client failed", err.Error())
		return
	}
	plan.applyAgencyClient(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *agencyClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agencyClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.GetAgencyClient(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read agency client failed", err.Error())
		return
	}
	state.applyAgencyClient(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *agencyClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan agencyClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.UpdateAgencyClient(ctx, plan.ID.ValueString(), plan.request())
	if err != nil {
		resp.Diagnostics.AddError("Update agency client failed", err.Error())
		return
	}
	plan.applyAgencyClient(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *agencyClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agencyClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteAgencyClient(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Delete agency client failed", err.Error())
		}
	}
}
func (m agencyClientModel) request() client.AgencyClientRequest {
	return client.AgencyClientRequest{Name: m.Name.ValueString(), Company: stringPtr(m.Company), Email: stringPtr(m.Email), Phone: stringPtr(m.Phone), Status: m.Status.ValueString(), BillingCurrency: m.BillingCurrency.ValueString(), Notes: stringPtr(m.Notes), TeamID: stringPtr(m.TeamID)}
}
func (m *agencyClientModel) applyAgencyClient(c *client.AgencyClient) {
	m.ID = types.StringValue(c.ID)
	m.Name = types.StringValue(c.Name)
	m.Company = stringValue(c.Company)
	m.Email = stringValue(c.Email)
	m.Phone = stringValue(c.Phone)
	m.Status = types.StringValue(c.Status)
	m.BillingCurrency = types.StringValue(c.BillingCurrency)
	m.Notes = stringValue(c.Notes)
	m.CreatedAt = timeString(c.CreatedAt)
}

func (r *agencyResourceAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agency_resource_assignment"
}
func (r *agencyResourceAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *agencyResourceAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "client_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"resource_type": schema.StringAttribute{Required: true, PlanModifiers: replaceString()}, "resource_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"label": schema.StringAttribute{Optional: true}, "team_id": schema.StringAttribute{Optional: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *agencyResourceAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agencyResourceAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.AssignAgencyResource(ctx, plan.ClientID.ValueString(), client.AgencyResourceRequest{ResourceType: plan.ResourceType.ValueString(), ResourceID: plan.ResourceID.ValueString(), Label: stringPtr(plan.Label), TeamID: stringPtr(plan.TeamID)})
	if err != nil {
		resp.Diagnostics.AddError("Assign agency resource failed", err.Error())
		return
	}
	plan.ID = types.StringValue(out.ID)
	plan.CreatedAt = timeString(out.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *agencyResourceAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agencyResourceAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *agencyResourceAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan agencyResourceAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *agencyResourceAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agencyResourceAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteAgencyResource(ctx, state.ClientID.ValueString(), state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Delete agency resource assignment failed", err.Error())
		}
	}
}
