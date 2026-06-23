package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type buildActionResource struct{ client *client.Client }
type deploymentActionResource struct{ client *client.Client }

type buildActionModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	BuildID   types.String `tfsdk:"build_id"`
	Status    types.String `tfsdk:"status"`
	Nonce     types.String `tfsdk:"nonce"`
}

type deploymentActionModel struct {
	ID           types.String `tfsdk:"id"`
	ProjectID    types.String `tfsdk:"project_id"`
	Action       types.String `tfsdk:"action"`
	DeploymentID types.String `tfsdk:"deployment_id"`
	ResultID     types.String `tfsdk:"result_id"`
	Message      types.String `tfsdk:"message"`
	Nonce        types.String `tfsdk:"nonce"`
}

func NewBuildActionResource() resource.Resource      { return &buildActionResource{} }
func NewDeploymentActionResource() resource.Resource { return &deploymentActionResource{} }

func (r *buildActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_build_action"
}
func (r *buildActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *buildActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":         schema.StringAttribute{Computed: true},
		"project_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"build_id":   schema.StringAttribute{Computed: true},
		"status":     schema.StringAttribute{Computed: true},
		"nonce":      schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
	}}
}
func (r *buildActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan buildActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	build, err := r.client.TriggerBuild(ctx, plan.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Trigger build failed", err.Error())
		return
	}
	plan.ID = types.StringValue("build-action:" + build.ID)
	plan.BuildID = types.StringValue(build.ID)
	plan.Status = types.StringValue(build.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *buildActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state buildActionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *buildActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan buildActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *buildActionResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *deploymentActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_action"
}
func (r *deploymentActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *deploymentActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":            schema.StringAttribute{Computed: true},
		"project_id":    schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"action":        schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"deployment_id": schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
		"result_id":     schema.StringAttribute{Computed: true},
		"message":       schema.StringAttribute{Computed: true},
		"nonce":         schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
	}}
}
func (r *deploymentActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.DeploymentAction(ctx, plan.ProjectID.ValueString(), plan.Action.ValueString(), plan.DeploymentID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Deployment action failed", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.Action.ValueString() + ":" + time.Now().UTC().Format(time.RFC3339Nano))
	plan.ResultID = types.StringValue(firstNonEmpty(result.BuildID, result.DeploymentID, result.OperationID))
	plan.Message = types.StringValue(result.Message)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *deploymentActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentActionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *deploymentActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan deploymentActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *deploymentActionResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func replaceString() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
