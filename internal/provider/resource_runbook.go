package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type runbookResource struct{ client *client.Client }
type runbookExecutionResource struct{ client *client.Client }

type runbookModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Slug                types.String `tfsdk:"slug"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Framework           types.String `tfsdk:"framework"`
	Command             types.String `tfsdk:"command"`
	ParametersJSON      types.String `tfsdk:"parameters_json"`
	Environments        types.List   `tfsdk:"environments"`
	RequiredPermissions types.List   `tfsdk:"required_permissions"`
	ApprovalRequired    types.Bool   `tfsdk:"approval_required"`
	Published           types.Bool   `tfsdk:"published"`
	VersionID           types.String `tfsdk:"version_id"`
	CreatedAt           types.String `tfsdk:"created_at"`
}

type runbookExecutionModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	VersionID         types.String `tfsdk:"version_id"`
	Parameters        types.Map    `tfsdk:"parameters"`
	ApprovalRequestID types.String `tfsdk:"approval_request_id"`
	Reason            types.String `tfsdk:"reason"`
	Status            types.String `tfsdk:"status"`
	Nonce             types.String `tfsdk:"nonce"`
}

func NewRunbookResource() resource.Resource          { return &runbookResource{} }
func NewRunbookExecutionResource() resource.Resource { return &runbookExecutionResource{} }

func (r *runbookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook"
}
func (r *runbookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *runbookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "project_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"slug": schema.StringAttribute{Required: true, PlanModifiers: replaceString()}, "name": schema.StringAttribute{Required: true},
		"description": schema.StringAttribute{Optional: true}, "framework": schema.StringAttribute{Optional: true},
		"command": schema.StringAttribute{Required: true}, "parameters_json": schema.StringAttribute{Optional: true},
		"environments":         schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"required_permissions": schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"approval_required":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"published":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"version_id":           schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
	}}
}
func (r *runbookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runbookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	out, err := r.client.CreateRunbook(ctx, plan.ProjectID.ValueString(), client.RunbookRequest{
		Slug: plan.Slug.ValueString(), Name: plan.Name.ValueString(), Description: plan.Description.ValueString(), Framework: stringPtr(plan.Framework),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create runbook failed", err.Error())
		return
	}
	plan.ID = types.StringValue(out.ID)
	plan.CreatedAt = timeString(out.CreatedAt)
	r.createVersion(ctx, &plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *runbookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runbookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *runbookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runbookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.createVersion(ctx, &plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *runbookResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
func (r *runbookResource) createVersion(ctx context.Context, plan *runbookModel, diags *diag.Diagnostics) {
	var envs []string
	var permissions []string
	diags.Append(plan.Environments.ElementsAs(ctx, &envs, false)...)
	diags.Append(plan.RequiredPermissions.ElementsAs(ctx, &permissions, false)...)
	if diags.HasError() {
		return
	}
	parameters := json.RawMessage("[]")
	if !plan.ParametersJSON.IsNull() && plan.ParametersJSON.ValueString() != "" {
		parameters = json.RawMessage(plan.ParametersJSON.ValueString())
	}
	result, err := r.client.CreateRunbookVersion(ctx, plan.ProjectID.ValueString(), plan.ID.ValueString(), client.RunbookVersionRequest{
		Command: plan.Command.ValueString(), Parameters: parameters, Environments: envs,
		RequiredPermissions: permissions, ApprovalRequired: plan.ApprovalRequired.ValueBool(), Published: plan.Published.ValueBool(),
	})
	if err != nil {
		diags.AddError("Create runbook version failed", err.Error())
		return
	}
	plan.VersionID = types.StringValue(result.OperationID)
}

func (r *runbookExecutionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook_execution"
}
func (r *runbookExecutionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *runbookExecutionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "project_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"version_id":          schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"parameters":          schema.MapAttribute{Optional: true, ElementType: types.StringType},
		"approval_request_id": schema.StringAttribute{Optional: true}, "reason": schema.StringAttribute{Optional: true},
		"status": schema.StringAttribute{Computed: true}, "nonce": schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
	}}
}
func (r *runbookExecutionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runbookExecutionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := mapFromTerraform(ctx, plan.Parameters, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.ExecuteRunbook(ctx, plan.ProjectID.ValueString(), plan.VersionID.ValueString(), client.RunbookExecutionRequest{
		Parameters: params, ApprovalRequestID: stringPtr(plan.ApprovalRequestID), Reason: plan.Reason.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Execute runbook failed", err.Error())
		return
	}
	plan.ID = types.StringValue(firstNonEmpty(result.ExecutionID, time.Now().UTC().Format(time.RFC3339Nano)))
	plan.Status = types.StringValue(result.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *runbookExecutionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runbookExecutionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *runbookExecutionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runbookExecutionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *runbookExecutionResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
