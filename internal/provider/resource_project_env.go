package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*projectEnvResource)(nil)
var _ resource.ResourceWithConfigure = (*projectEnvResource)(nil)
var _ resource.ResourceWithImportState = (*projectEnvResource)(nil)

func NewProjectEnvResource() resource.Resource { return &projectEnvResource{} }

type projectEnvResource struct{ client *client.Client }

type projectEnvModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Environment types.String `tfsdk:"environment"`
	Variables   types.Map    `tfsdk:"variables"`
}

func (r *projectEnvResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_env"
}

func (r *projectEnvResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *projectEnvResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the declared environment variables for one StackShift project environment. Other variables on the environment (including platform-injected values such as managed-database URLs) are preserved on write and ignored on read.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true},
			"project_id":  schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"environment": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("production"), PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"variables":   schema.MapAttribute{Required: true, Sensitive: true, ElementType: types.StringType},
		},
	}
}

func (r *projectEnvResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectEnvModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Environment.IsNull() || plan.Environment.ValueString() == "" {
		plan.Environment = types.StringValue("production")
	}
	managed := mapFromTerraform(ctx, plan.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.write(ctx, plan.ProjectID.ValueString(), plan.Environment.ValueString(), managed, nil, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(plan.ProjectID.ValueString() + ":" + plan.Environment.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectEnvResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectEnvModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	managed := mapFromTerraform(ctx, state.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	env, err := r.client.GetProjectEnv(ctx, state.ProjectID.ValueString(), state.Environment.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift env vars failed", err.Error())
		return
	}
	live := map[string]string{}
	for _, item := range env {
		if item.Source == "global" {
			continue
		}
		live[item.Key] = item.Value
	}
	values := map[string]string{}
	for key := range managed {
		if v, ok := live[key]; ok {
			values[key] = v
		}
	}
	state.Variables = terraformMap(ctx, values, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectEnvResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectEnvModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Environment.IsNull() || plan.Environment.ValueString() == "" {
		plan.Environment = types.StringValue("production")
	}
	managed := mapFromTerraform(ctx, plan.Variables, &resp.Diagnostics)
	prior := mapFromTerraform(ctx, state.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	drop := map[string]struct{}{}
	for key := range prior {
		if _, ok := managed[key]; !ok {
			drop[key] = struct{}{}
		}
	}
	if !r.write(ctx, plan.ProjectID.ValueString(), plan.Environment.ValueString(), managed, drop, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(plan.ProjectID.ValueString() + ":" + plan.Environment.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectEnvResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectEnvModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	managed := mapFromTerraform(ctx, state.Variables, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	drop := map[string]struct{}{}
	for key := range managed {
		drop[key] = struct{}{}
	}
	if !r.write(ctx, state.ProjectID.ValueString(), state.Environment.ValueString(), nil, drop, &resp.Diagnostics) {
		return
	}
}

func (r *projectEnvResource) write(ctx context.Context, projectID, environment string, managed map[string]string, drop map[string]struct{}, diags *diag.Diagnostics) bool {
	existing, err := r.client.GetProjectEnv(ctx, projectID, environment)
	if err != nil && !notFound(err) {
		diags.AddError("Read StackShift env vars failed", err.Error())
		return false
	}
	byKey := map[string]client.EnvVarInput{}
	for _, item := range existing {
		if item.Source == "global" {
			continue
		}
		if _, isManaged := managed[item.Key]; isManaged {
			continue
		}
		if _, dropped := drop[item.Key]; dropped {
			continue
		}
		value := item.Value
		byKey[item.Key] = client.EnvVarInput{Key: item.Key, Value: &value, IsSecret: item.IsSecret}
	}
	for key, value := range managed {
		v := value
		byKey[key] = client.EnvVarInput{Key: key, Value: &v, IsSecret: true}
	}
	inputs := make([]client.EnvVarInput, 0, len(byKey))
	for _, in := range byKey {
		inputs = append(inputs, in)
	}
	if err := r.client.SetProjectEnv(ctx, projectID, environment, inputs); err != nil {
		diags.AddError("Set StackShift env vars failed", err.Error())
		return false
	}
	return true
}

func (r *projectEnvResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment"), parts[1])...)
}
