package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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

var _ resource.ResourceWithConfigure = (*dedicatedParameterGroupAttachmentResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedParameterGroupAttachmentResource)(nil)

func NewDedicatedParameterGroupAttachmentResource() resource.Resource {
	return &dedicatedParameterGroupAttachmentResource{}
}

type dedicatedParameterGroupAttachmentResource struct{ client *client.Client }

type dedicatedParameterGroupAttachmentModel struct {
	ID               types.String   `tfsdk:"id"`
	DatabaseID       types.String   `tfsdk:"database_id"`
	ParameterGroupID types.String   `tfsdk:"parameter_group_id"`
	RestartToApply   types.Bool     `tfsdk:"restart_to_apply"`
	Applied          types.Bool     `tfsdk:"applied"`
	PendingRestart   types.List     `tfsdk:"pending_restart"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *dedicatedParameterGroupAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_parameter_group_attachment"
}

func (r *dedicatedParameterGroupAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedParameterGroupAttachmentResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches a parameter group to a Dedicated database. The database must be running, and the group must be for its engine and version. Destroying this resource detaches the group and returns the database to its default settings.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"database_id":        schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"parameter_group_id": schema.StringAttribute{Required: true, Description: "A stackshift_database_parameter_group id. Changes in place."},
			"restart_to_apply": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Restart the database right after the group is attached or changed, so settings that need a restart take effect. Connections drop briefly."},
			"applied": schema.BoolAttribute{Computed: true, Description: "Whether the database is running with the group's settings."},
			"pending_restart": schema.ListAttribute{Computed: true, ElementType: types.StringType,
				Description: "Settings that are waiting for a restart."},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}

func (r *dedicatedParameterGroupAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedParameterGroupAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.DatabaseID
	timeout, diags := plan.Timeouts.Create(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !r.attach(ctx, &plan, timeout, true, plan.RestartToApply.ValueBool(), &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedParameterGroupAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedParameterGroupAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.GetDedicatedDatabase(ctx, state.DatabaseID.ValueString()); notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	} else if err != nil {
		resp.Diagnostics.AddError("Read StackShift database parameters failed", dedicatedErrorDetail(err))
		return
	}
	parameters, err := r.client.GetDedicatedParameters(ctx, state.DatabaseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database parameters failed", dedicatedErrorDetail(err))
		return
	}
	if parameters.Group == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.ParameterGroupID = types.StringValue(parameters.Group.ID)
	state.applyParameters(ctx, parameters, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedParameterGroupAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dedicatedParameterGroupAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupChanged := !plan.ParameterGroupID.Equal(state.ParameterGroupID)
	restart := plan.RestartToApply.ValueBool() && (groupChanged || !state.RestartToApply.ValueBool())
	if !groupChanged && !restart {
		plan.Applied, plan.PendingRestart = state.Applied, state.PendingRestart
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	timeout, diags := plan.Timeouts.Update(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !r.attach(ctx, &plan, timeout, groupChanged, restart, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedParameterGroupAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedParameterGroupAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.SetDedicatedParameterGroup(ctx, state.DatabaseID.ValueString(), nil); err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Detach StackShift parameter group failed", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedParameterGroupAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("restart_to_apply"), false)...)
}

func (r *dedicatedParameterGroupAttachmentResource) attach(ctx context.Context, plan *dedicatedParameterGroupAttachmentModel, timeout time.Duration, setGroup, restart bool, diags *diag.Diagnostics) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	databaseID := plan.DatabaseID.ValueString()
	groupID := plan.ParameterGroupID.ValueString()
	if err := waitDedicatedRunning(ctx, r.client, databaseID); err != nil {
		diags.AddError("The database isn't ready for a parameter group", dedicatedErrorDetail(err))
		return false
	}
	if setGroup {
		if err := r.client.SetDedicatedParameterGroup(ctx, databaseID, &groupID); err != nil {
			diags.AddError("Attach StackShift parameter group failed", dedicatedErrorDetail(err))
			return false
		}
	}
	if restart {
		if err := r.client.ApplyDedicatedParameters(ctx, databaseID); err != nil {
			diags.AddError("Restart for StackShift parameters failed", dedicatedErrorDetail(err))
			return false
		}
		err := pollDedicated(ctx, func(ctx context.Context) (bool, error) {
			parameters, err := r.client.GetDedicatedParameters(ctx, databaseID)
			if err != nil {
				return false, err
			}
			if parameters.Error != "" {
				return false, fmt.Errorf("%s", parameters.Error)
			}
			return parameters.Applied && len(parameters.PendingRestart) == 0, nil
		})
		if err != nil {
			diags.AddError("StackShift parameters weren't applied", dedicatedErrorDetail(err))
			return false
		}
		if err := waitDedicatedRunning(ctx, r.client, databaseID); err != nil {
			diags.AddError("The database didn't come back after restarting", dedicatedErrorDetail(err))
			return false
		}
	}
	plan.Applied, plan.PendingRestart = types.BoolNull(), types.ListNull(types.StringType)
	if parameters, err := r.client.GetDedicatedParameters(ctx, databaseID); err == nil {
		plan.applyParameters(ctx, parameters, diags)
	}
	return true
}

func (m *dedicatedParameterGroupAttachmentModel) applyParameters(ctx context.Context, parameters *client.DedicatedParameters, diags *diag.Diagnostics) {
	pending := parameters.PendingRestart
	if pending == nil {
		pending = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, pending)
	diags.Append(d...)
	m.Applied, m.PendingRestart = types.BoolValue(parameters.Applied), list
}
