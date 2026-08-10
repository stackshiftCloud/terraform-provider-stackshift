package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func NewBYOCSnapshotResource() resource.Resource { return &byocSnapshotResource{} }

type byocSnapshotResource struct{ client *client.Client }

type byocSnapshotModel struct {
	ID                   types.String `tfsdk:"id"`
	NodeID               types.String `tfsdk:"node_id"`
	VolumeID             types.String `tfsdk:"volume_id"`
	Name                 types.String `tfsdk:"name"`
	IdempotencyKey       types.String `tfsdk:"idempotency_key"`
	DeleteIdempotencyKey types.String `tfsdk:"delete_idempotency_key"`
	Provider             types.String `tfsdk:"provider"`
	ProviderSnapshotID   types.String `tfsdk:"provider_snapshot_id"`
	SizeGB               types.Int64  `tfsdk:"size_gb"`
	State                types.String `tfsdk:"state"`
	FailureReason        types.String `tfsdk:"failure_reason"`
}

func (r *byocSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_snapshot"
}

func (r *byocSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *byocSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := storageReplace()
	resp.Schema = schema.Schema{MarkdownDescription: "Creates and safely deletes an inventoried snapshot of a BYOCloud volume.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "node_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
		"volume_id": schema.StringAttribute{Required: true, PlanModifiers: replace}, "name": schema.StringAttribute{Required: true, PlanModifiers: replace},
		"idempotency_key": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: replace}, "delete_idempotency_key": schema.StringAttribute{Computed: true},
		"provider": schema.StringAttribute{Computed: true}, "provider_snapshot_id": schema.StringAttribute{Computed: true},
		"size_gb": schema.Int64Attribute{Computed: true}, "state": schema.StringAttribute{Computed: true}, "failure_reason": schema.StringAttribute{Computed: true},
	}}
}

func (r *byocSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan byocSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(plan.IdempotencyKey.ValueString(), "tf-byoc-snapshot-create", plan.NodeID.ValueString(), plan.VolumeID.ValueString(), plan.Name.ValueString())
	snapshot, err := r.client.CreateBYOCSnapshot(ctx, plan.VolumeID.ValueString(), key, plan.Name.ValueString())
	if err == nil {
		snapshot, err = waitForSnapshot(ctx, r.client, plan.NodeID.ValueString(), snapshot.ID, false)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create BYOCloud snapshot", err.Error())
		return
	}
	deleteKey := deterministicIdempotencyKey("tf-byoc-snapshot-delete", snapshot.ID)
	plan.IdempotencyKey, plan.DeleteIdempotencyKey = types.StringValue(key), types.StringValue(deleteKey)
	plan.apply(snapshot)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byocSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	snapshot, err := findSnapshot(ctx, r.client, state.NodeID.ValueString(), state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read BYOCloud snapshot", err.Error())
		return
	}
	state.apply(snapshot)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *byocSnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan byocSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byocSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(state.DeleteIdempotencyKey.ValueString(), "tf-byoc-snapshot-delete", state.ID.ValueString())
	err := r.client.DeleteBYOCSnapshot(ctx, state.ID.ValueString(), key)
	if err == nil {
		_, err = waitForSnapshot(ctx, r.client, state.NodeID.ValueString(), state.ID.ValueString(), true)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete BYOCloud snapshot", err.Error())
	}
}

func (r *byocSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importBYOCComposite(ctx, req.ID, resp)
}

func findSnapshot(ctx context.Context, api *client.Client, nodeID, id string) (*client.BYOCSnapshot, error) {
	summary, err := api.GetBYOCNodeResources(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	for i := range summary.Snapshots {
		if summary.Snapshots[i].ID == id {
			return &summary.Snapshots[i], nil
		}
	}
	return nil, client.ErrNotFound
}

func waitForSnapshot(ctx context.Context, api *client.Client, nodeID, id string, absent bool) (*client.BYOCSnapshot, error) {
	for {
		snapshot, err := findSnapshot(ctx, api, nodeID, id)
		if absent && errors.Is(err, client.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !absent && snapshot.State == "available" {
			return snapshot, nil
		}
		if snapshot.State == "failed" {
			return nil, fmt.Errorf("snapshot operation failed: %s", snapshot.FailureReason)
		}
		if err := waitBYOCPoll(ctx); err != nil {
			return nil, err
		}
	}
}

func (m *byocSnapshotModel) apply(snapshot *client.BYOCSnapshot) {
	m.ID = types.StringValue(snapshot.ID)
	m.NodeID = types.StringValue(snapshot.NodeID)
	m.VolumeID = types.StringValue(snapshot.VolumeID)
	m.Name = types.StringValue(snapshot.Name)
	m.Provider = types.StringValue(snapshot.Provider)
	m.ProviderSnapshotID = optionalBYOCString(snapshot.ProviderSnapshotID)
	m.SizeGB = types.Int64Value(snapshot.SizeGB)
	m.State = types.StringValue(snapshot.State)
	m.FailureReason = optionalBYOCString(snapshot.FailureReason)
}
