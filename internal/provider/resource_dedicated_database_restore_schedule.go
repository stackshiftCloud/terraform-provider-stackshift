package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseRestoreScheduleResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseRestoreScheduleResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dedicatedDatabaseRestoreScheduleResource)(nil)

const restoreScheduleMaxKeep = 30

var restoreScheduleKinds = []string{"daily", "weekly", "monthly"}

var restoreScheduleType = types.ObjectType{AttrTypes: map[string]attr.Type{"enabled": types.BoolType, "keep": types.Int64Type}}

func NewDedicatedDatabaseRestoreScheduleResource() resource.Resource {
	return &dedicatedDatabaseRestoreScheduleResource{}
}

type dedicatedDatabaseRestoreScheduleResource struct{ client *client.Client }

type dedicatedDatabaseRestoreScheduleModel struct {
	ID         types.String `tfsdk:"id"`
	DatabaseID types.String `tfsdk:"database_id"`
	Daily      types.Object `tfsdk:"daily"`
	Weekly     types.Object `tfsdk:"weekly"`
	Monthly    types.Object `tfsdk:"monthly"`
}

type restoreScheduleModel struct {
	Enabled types.Bool  `tfsdk:"enabled"`
	Keep    types.Int64 `tfsdk:"keep"`
}

func (m *dedicatedDatabaseRestoreScheduleModel) schedules() map[string]*types.Object {
	return map[string]*types.Object{"daily": &m.Daily, "weekly": &m.Weekly, "monthly": &m.Monthly}
}

func (r *dedicatedDatabaseRestoreScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_restore_schedule"
}

func (r *dedicatedDatabaseRestoreScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func restoreScheduleAttribute(description string) schema.Attribute {
	return schema.ObjectAttribute{
		Optional: true, Computed: true, AttributeTypes: restoreScheduleType.AttrTypes, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
		Description: fmt.Sprintf("%s { enabled = whether this schedule takes restore points, keep = how many to keep, 1 to %d }.", description, restoreScheduleMaxKeep),
	}
}

func (r *dedicatedDatabaseRestoreScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "How often a Dedicated database takes automatic fast restore points, and how many of each it keeps. Every database has a daily, a weekly and a monthly schedule. Schedules you leave out keep their current settings. Destroying this resource only stops Terraform from managing the schedules; it doesn't change them.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"daily":       restoreScheduleAttribute("The daily schedule. Changes in place."),
			"weekly":      restoreScheduleAttribute("The weekly schedule. Changes in place."),
			"monthly":     restoreScheduleAttribute("The monthly schedule. Changes in place."),
		},
	}
}

func (r *dedicatedDatabaseRestoreScheduleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, kind := range restoreScheduleKinds {
		var object types.Object
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(kind), &object)...)
		if object.IsNull() || object.IsUnknown() {
			continue
		}
		var schedule restoreScheduleModel
		resp.Diagnostics.Append(object.As(ctx, &schedule, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() || schedule.Keep.IsUnknown() || schedule.Keep.IsNull() {
			continue
		}
		if keep := schedule.Keep.ValueInt64(); keep < 1 || keep > restoreScheduleMaxKeep {
			resp.Diagnostics.AddAttributeError(path.Root(kind).AtName("keep"), "Unsupported value", fmt.Sprintf("Keep between 1 and %d restore points.", restoreScheduleMaxKeep))
		}
	}
}

func (r *dedicatedDatabaseRestoreScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseRestoreScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.DatabaseID
	if err := waitDedicatedRunning(ctx, r.client, plan.DatabaseID.ValueString()); err != nil {
		resp.Diagnostics.AddError("The database isn't ready for restore point schedules", dedicatedErrorDetail(err))
		return
	}
	r.save(ctx, &plan, dedicatedDatabaseRestoreScheduleModel{}, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseRestoreScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseRestoreScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	points, err := r.client.ListDedicatedRestorePoints(ctx, state.DatabaseID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift restore point schedules failed", dedicatedErrorDetail(err))
		return
	}
	state.apply(points.Schedules, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseRestoreScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dedicatedDatabaseRestoreScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.save(ctx, &plan, state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseRestoreScheduleResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func (r *dedicatedDatabaseRestoreScheduleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), req.ID)...)
}

func (r *dedicatedDatabaseRestoreScheduleResource) save(ctx context.Context, plan *dedicatedDatabaseRestoreScheduleModel, state dedicatedDatabaseRestoreScheduleModel, diags *diag.Diagnostics) {
	changes := map[string]client.DedicatedRestoreScheduleChange{}
	current := state.schedules()
	for kind, planned := range plan.schedules() {
		if planned.IsNull() || planned.IsUnknown() || planned.Equal(*current[kind]) {
			continue
		}
		var schedule restoreScheduleModel
		diags.Append(planned.As(ctx, &schedule, basetypes.ObjectAsOptions{})...)
		changes[kind] = client.DedicatedRestoreScheduleChange{Enabled: schedule.Enabled.ValueBool(), Keep: schedule.Keep.ValueInt64()}
	}
	if diags.HasError() {
		return
	}
	databaseID := plan.DatabaseID.ValueString()
	var schedules []client.DedicatedRestoreSchedule
	var err error
	if len(changes) > 0 {
		schedules, err = r.client.UpdateDedicatedRestoreSchedules(ctx, databaseID, changes)
	} else {
		var points *client.DedicatedRestorePoints
		if points, err = r.client.ListDedicatedRestorePoints(ctx, databaseID); err == nil {
			schedules = points.Schedules
		}
	}
	if err != nil {
		diags.AddError("Update StackShift restore point schedules failed", dedicatedErrorDetail(err))
		return
	}
	plan.apply(schedules, diags)
}

func (m *dedicatedDatabaseRestoreScheduleModel) apply(schedules []client.DedicatedRestoreSchedule, diags *diag.Diagnostics) {
	current := m.schedules()
	for _, schedule := range schedules {
		target, ok := current[schedule.Kind]
		if !ok {
			continue
		}
		object, d := types.ObjectValue(restoreScheduleType.AttrTypes, map[string]attr.Value{
			"enabled": types.BoolValue(schedule.Enabled), "keep": types.Int64Value(schedule.Keep),
		})
		diags.Append(d...)
		*target = object
	}
	for _, target := range current {
		if target.IsUnknown() {
			*target = types.ObjectNull(restoreScheduleType.AttrTypes)
		}
	}
}
