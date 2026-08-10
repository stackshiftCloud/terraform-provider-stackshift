package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func NewBYOCVolumeResource() resource.Resource { return &byocVolumeResource{} }

type byocVolumeResource struct{ client *client.Client }

type byocVolumeModel struct {
	ID                   types.String `tfsdk:"id"`
	NodeID               types.String `tfsdk:"node_id"`
	Name                 types.String `tfsdk:"name"`
	SizeGB               types.Int64  `tfsdk:"size_gb"`
	MountPath            types.String `tfsdk:"mount_path"`
	Filesystem           types.String `tfsdk:"filesystem"`
	IdempotencyKey       types.String `tfsdk:"idempotency_key"`
	DeleteIdempotencyKey types.String `tfsdk:"delete_idempotency_key"`
	Provider             types.String `tfsdk:"provider"`
	ProviderVolumeID     types.String `tfsdk:"provider_volume_id"`
	Region               types.String `tfsdk:"region"`
	State                types.String `tfsdk:"state"`
	FailureReason        types.String `tfsdk:"failure_reason"`
	SnapshotCount        types.Int64  `tfsdk:"snapshot_count"`
}

func storageReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *byocVolumeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_volume"
}
func (r *byocVolumeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *byocVolumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := storageReplace()
	resp.Schema = schema.Schema{MarkdownDescription: "Creates, attaches, inventories, and safely deletes a provider-backed BYOCloud volume.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "node_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
		"name": schema.StringAttribute{Required: true, PlanModifiers: replace}, "size_gb": schema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"mount_path": schema.StringAttribute{Optional: true, PlanModifiers: replace}, "filesystem": schema.StringAttribute{Optional: true, PlanModifiers: replace},
		"idempotency_key": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: replace}, "delete_idempotency_key": schema.StringAttribute{Computed: true},
		"provider": schema.StringAttribute{Computed: true}, "provider_volume_id": schema.StringAttribute{Computed: true}, "region": schema.StringAttribute{Computed: true},
		"state": schema.StringAttribute{Computed: true}, "failure_reason": schema.StringAttribute{Computed: true}, "snapshot_count": schema.Int64Attribute{Computed: true},
	}}
}

func ensureKey(current, prefix string, identity ...string) string {
	if value := strings.TrimSpace(current); value != "" {
		return value
	}
	return deterministicIdempotencyKey(prefix, identity...)
}

func (r *byocVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan byocVolumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(plan.IdempotencyKey.ValueString(), "tf-byoc-volume-create", plan.NodeID.ValueString(), plan.Name.ValueString(), fmt.Sprint(plan.SizeGB.ValueInt64()), plan.MountPath.ValueString(), plan.Filesystem.ValueString())
	volume, err := r.client.CreateBYOCVolume(ctx, plan.NodeID.ValueString(), key, map[string]any{"name": plan.Name.ValueString(), "size_gb": plan.SizeGB.ValueInt64(), "mount_path": plan.MountPath.ValueString(), "filesystem": plan.Filesystem.ValueString()})
	if err == nil {
		volume, err = waitForVolume(ctx, r.client, plan.NodeID.ValueString(), volume.ID, false)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create BYOCloud volume", err.Error())
		return
	}
	deleteKey := deterministicIdempotencyKey("tf-byoc-volume-delete", volume.ID)
	plan.IdempotencyKey, plan.DeleteIdempotencyKey = types.StringValue(key), types.StringValue(deleteKey)
	plan.apply(volume)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocVolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byocVolumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	volume, err := findVolume(ctx, r.client, state.NodeID.ValueString(), state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read BYOCloud volume", err.Error())
		return
	}
	state.apply(volume)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *byocVolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan byocVolumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byocVolumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(state.DeleteIdempotencyKey.ValueString(), "tf-byoc-volume-delete", state.ID.ValueString())
	err := r.client.DeleteBYOCVolume(ctx, state.ID.ValueString(), key)
	if err == nil {
		_, err = waitForVolume(ctx, r.client, state.NodeID.ValueString(), state.ID.ValueString(), true)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete BYOCloud volume", err.Error())
	}
}

func (r *byocVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importBYOCComposite(ctx, req.ID, resp)
}

func importBYOCComposite(ctx context.Context, id string, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(id, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid BYOCloud import ID", "Use NODE_ID:RESOURCE_ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("node_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func findVolume(ctx context.Context, api *client.Client, nodeID, id string) (*client.BYOCVolume, error) {
	summary, err := api.GetBYOCNodeResources(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	for i := range summary.Volumes {
		if summary.Volumes[i].ID == id {
			return &summary.Volumes[i], nil
		}
	}
	return nil, client.ErrNotFound
}
func waitForVolume(ctx context.Context, api *client.Client, nodeID, id string, absent bool) (*client.BYOCVolume, error) {
	for {
		volume, err := findVolume(ctx, api, nodeID, id)
		if absent && errors.Is(err, client.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !absent && volume.State == "attached" {
			return volume, nil
		}
		if volume.State == "failed" {
			return nil, fmt.Errorf("volume operation failed: %s", volume.FailureReason)
		}
		if err := waitBYOCPoll(ctx); err != nil {
			return nil, err
		}
	}
}
func waitBYOCPoll(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		return nil
	}
}

func (m *byocVolumeModel) apply(v *client.BYOCVolume) {
	m.ID = types.StringValue(v.ID)
	m.NodeID = types.StringValue(v.NodeID)
	m.Name = types.StringValue(v.Name)
	m.SizeGB = types.Int64Value(v.SizeGB)
	m.MountPath = optionalBYOCString(v.MountPath)
	m.Filesystem = optionalBYOCString(v.Filesystem)
	m.Provider = types.StringValue(v.Provider)
	m.ProviderVolumeID = optionalBYOCString(v.ProviderVolumeID)
	m.Region = types.StringValue(v.Region)
	m.State = types.StringValue(v.State)
	m.FailureReason = optionalBYOCString(v.FailureReason)
	m.SnapshotCount = types.Int64Value(v.SnapshotCount)
}
