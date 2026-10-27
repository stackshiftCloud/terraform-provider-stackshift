package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseSnapshotResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseSnapshotResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dedicatedDatabaseSnapshotResource)(nil)

func NewDedicatedDatabaseSnapshotResource() resource.Resource {
	return &dedicatedDatabaseSnapshotResource{}
}

type dedicatedDatabaseSnapshotResource struct{ client *client.Client }

type dedicatedDatabaseSnapshotModel struct {
	ID         types.String   `tfsdk:"id"`
	DatabaseID types.String   `tfsdk:"database_id"`
	Name       types.String   `tfsdk:"name"`
	Kind       types.String   `tfsdk:"kind"`
	Status     types.String   `tfsdk:"status"`
	SizeBytes  types.Int64    `tfsdk:"size_bytes"`
	CreatedAt  types.String   `tfsdk:"created_at"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

func (r *dedicatedDatabaseSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_snapshot"
}

func (r *dedicatedDatabaseSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedDatabaseSnapshotResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A snapshot of a Dedicated database that is kept until you delete it. The database must be running. Use its id as restore_from.backup_id to restore it into a new database.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"name":        schema.StringAttribute{Optional: true, PlanModifiers: replaceString(), Description: "Up to 100 characters. Changing this replaces the snapshot."},
			"kind":        schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"status":      schema.StringAttribute{Computed: true},
			"size_bytes":  schema.Int64Attribute{Computed: true},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"timeouts":    timeouts.Attributes(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

func (r *dedicatedDatabaseSnapshotResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateUnpaddedName(ctx, req.Config, "name", &resp.Diagnostics)
}

func (r *dedicatedDatabaseSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, timeoutDiags := plan.Timeouts.Create(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(timeoutDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	databaseID := plan.DatabaseID.ValueString()
	if err := waitDedicatedRunning(ctx, r.client, databaseID, createTimeout); err != nil {
		resp.Diagnostics.AddError("The database isn't ready for a snapshot", dedicatedErrorDetail(err))
		return
	}
	snapshot, err := r.client.CreateDedicatedSnapshot(ctx, databaseID, plannedString(plan.Name))
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift snapshot failed", dedicatedErrorDetail(err))
		return
	}
	plan.apply(snapshot)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	err = pollDedicated(ctx, createTimeout, func(ctx context.Context) (bool, error) {
		current, err := r.client.GetDedicatedSnapshot(ctx, databaseID, snapshot.ID)
		if err != nil {
			return false, err
		}
		plan.apply(current)
		switch current.Status {
		case "available":
			return true, nil
		case "failed":
			message := "the snapshot failed"
			if current.FailureMessage != nil {
				message = *current.FailureMessage
			}
			return false, fmt.Errorf("%s", message)
		}
		return false, nil
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("StackShift snapshot didn't finish", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	snapshot, err := r.client.GetDedicatedSnapshot(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if notFound(err) || err == nil && (snapshot.Status == "deleting" || snapshot.Status == "deleted") {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift snapshot failed", dedicatedErrorDetail(err))
		return
	}
	state.apply(snapshot)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseSnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dedicatedDatabaseSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabaseSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, timeoutDiags := state.Timeouts.Delete(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(timeoutDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	databaseID, id := state.DatabaseID.ValueString(), state.ID.ValueString()
	if err := r.client.DeleteDedicatedSnapshot(ctx, databaseID, id); err != nil {
		resp.Diagnostics.AddError("Delete StackShift snapshot failed", dedicatedErrorDetail(err))
		return
	}
	err := pollDedicated(ctx, deleteTimeout, func(ctx context.Context) (bool, error) {
		_, err := r.client.GetDedicatedSnapshot(ctx, databaseID, id)
		if notFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		resp.Diagnostics.AddError("StackShift snapshot wasn't deleted", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Use database_id:snapshot_id. "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *dedicatedDatabaseSnapshotModel) apply(snapshot *client.DedicatedBackupSet) {
	m.ID = types.StringValue(snapshot.ID)
	if snapshot.Name != nil {
		m.Name = nonEmptyString(*snapshot.Name)
	}
	m.Kind, m.Status = types.StringValue(snapshot.Kind), types.StringValue(snapshot.Status)
	m.SizeBytes, m.CreatedAt = types.Int64Value(snapshot.SizeBytes), nonEmptyString(snapshot.CreatedAt)
}
