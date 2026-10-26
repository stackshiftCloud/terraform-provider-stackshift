package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseRestorePointResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseRestorePointResource)(nil)

func NewDedicatedDatabaseRestorePointResource() resource.Resource {
	return &dedicatedDatabaseRestorePointResource{}
}

type dedicatedDatabaseRestorePointResource struct{ client *client.Client }

type dedicatedDatabaseRestorePointModel struct {
	ID         types.String `tfsdk:"id"`
	DatabaseID types.String `tfsdk:"database_id"`
	Name       types.String `tfsdk:"name"`
	Kind       types.String `tfsdk:"kind"`
	Status     types.String `tfsdk:"status"`
	SizeGB     types.Int64  `tfsdk:"size_gb"`
	TakenAt    types.String `tfsdk:"taken_at"`
}

func (r *dedicatedDatabaseRestorePointResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_restore_point"
}

func (r *dedicatedDatabaseRestorePointResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedDatabaseRestorePointResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A fast restore point of a Dedicated database, taken on demand and kept until you delete it. The database must be running and fast restore points must be available for it. You can keep a limited number of your own.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"name":        schema.StringAttribute{Optional: true, PlanModifiers: replaceString(), Description: "Up to 100 characters. Changing this replaces the restore point."},
			"kind":        schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"status":      schema.StringAttribute{Computed: true},
			"size_gb":     schema.Int64Attribute{Computed: true},
			"taken_at":    schema.StringAttribute{Computed: true},
		},
	}
}

func (r *dedicatedDatabaseRestorePointResource) find(ctx context.Context, databaseID, id string) (*client.DedicatedRestorePoint, error) {
	points, err := r.client.ListDedicatedRestorePoints(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	for i := range points.RestorePoints {
		if points.RestorePoints[i].ID == id {
			return &points.RestorePoints[i], nil
		}
	}
	return nil, client.ErrNotFound
}

func (r *dedicatedDatabaseRestorePointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseRestorePointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	databaseID := plan.DatabaseID.ValueString()
	if err := waitDedicatedRunning(ctx, r.client, databaseID); err != nil {
		resp.Diagnostics.AddError("The database isn't ready for a restore point", dedicatedErrorDetail(err))
		return
	}
	point, err := r.client.CreateDedicatedRestorePoint(ctx, databaseID, plannedString(plan.Name))
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift restore point failed", dedicatedErrorDetail(err))
		return
	}
	plan.apply(point)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	err = pollDedicated(ctx, func(ctx context.Context) (bool, error) {
		current, err := r.find(ctx, databaseID, point.ID)
		if err != nil {
			return false, err
		}
		plan.apply(current)
		switch current.Status {
		case "available":
			return true, nil
		case "failed":
			message := "the restore point failed"
			if current.FailureMessage != nil {
				message = *current.FailureMessage
			}
			return false, fmt.Errorf("%s", message)
		}
		return false, nil
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("StackShift restore point didn't finish", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseRestorePointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseRestorePointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	point, err := r.find(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if notFound(err) || err == nil && (point.Status == "deleting" || point.Status == "deleted") {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift restore point failed", dedicatedErrorDetail(err))
		return
	}
	state.apply(point)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseRestorePointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dedicatedDatabaseRestorePointModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseRestorePointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabaseRestorePointModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	databaseID, id := state.DatabaseID.ValueString(), state.ID.ValueString()
	if err := r.client.DeleteDedicatedRestorePoint(ctx, databaseID, id); err != nil {
		resp.Diagnostics.AddError("Delete StackShift restore point failed", dedicatedErrorDetail(err))
		return
	}
	err := pollDedicated(ctx, func(ctx context.Context) (bool, error) {
		point, err := r.find(ctx, databaseID, id)
		if notFound(err) || err == nil && point.Status == "deleted" {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		resp.Diagnostics.AddError("StackShift restore point wasn't deleted", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseRestorePointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Use database_id:restore_point_id. "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *dedicatedDatabaseRestorePointModel) apply(point *client.DedicatedRestorePoint) {
	m.ID = types.StringValue(point.ID)
	if point.Name != nil {
		m.Name = nonEmptyString(*point.Name)
	}
	m.Kind, m.Status, m.SizeGB = types.StringValue(point.Kind), types.StringValue(point.Status), types.Int64Value(point.SizeGB)
	m.TakenAt = types.StringNull()
	if point.TakenAt != nil {
		m.TakenAt = nonEmptyString(*point.TakenAt)
	}
}
