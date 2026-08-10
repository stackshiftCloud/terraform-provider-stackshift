package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

func NewBYOCStaticIPResource() resource.Resource { return &byocStaticIPResource{} }

type byocStaticIPResource struct{ client *client.Client }

type byocStaticIPModel struct {
	ID                   types.String `tfsdk:"id"`
	NodeID               types.String `tfsdk:"node_id"`
	IdempotencyKey       types.String `tfsdk:"idempotency_key"`
	DeleteIdempotencyKey types.String `tfsdk:"delete_idempotency_key"`
	ProviderResourceID   types.String `tfsdk:"provider_resource_id"`
	IPAddress            types.String `tfsdk:"ip_address"`
	Status               types.String `tfsdk:"status"`
	Assigned             types.Bool   `tfsdk:"assigned"`
	LastSyncedAt         types.String `tfsdk:"last_synced_at"`
}

func (r *byocStaticIPResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_static_ip"
}

func (r *byocStaticIPResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *byocStaticIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := storageReplace()
	resp.Schema = schema.Schema{MarkdownDescription: "Allocates a stable public IP for a BYOCloud node and releases it through the durable provider operation on destroy.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "node_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
		"idempotency_key": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: replace}, "delete_idempotency_key": schema.StringAttribute{Computed: true},
		"provider_resource_id": schema.StringAttribute{Computed: true}, "ip_address": schema.StringAttribute{Computed: true},
		"status": schema.StringAttribute{Computed: true}, "assigned": schema.BoolAttribute{Computed: true}, "last_synced_at": schema.StringAttribute{Computed: true},
	}}
}

func (r *byocStaticIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan byocStaticIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(plan.IdempotencyKey.ValueString(), "tf-byoc-static-ip-create", plan.NodeID.ValueString())
	address, err := r.client.AllocateBYOCStaticIP(ctx, plan.NodeID.ValueString(), key)
	if err == nil {
		address, err = waitForStaticIP(ctx, r.client, plan.NodeID.ValueString(), false)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to allocate BYOCloud static IP", err.Error())
		return
	}
	deleteKey := deterministicIdempotencyKey("tf-byoc-static-ip-delete", plan.NodeID.ValueString())
	plan.IdempotencyKey, plan.DeleteIdempotencyKey = types.StringValue(key), types.StringValue(deleteKey)
	plan.apply(address)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocStaticIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byocStaticIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	summary, err := r.client.GetBYOCNodeResources(ctx, state.NodeID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read BYOCloud static IP", err.Error())
		return
	}
	if staticIPAbsent(summary.StaticIP) {
		resp.State.RemoveResource(ctx)
		return
	}
	state.apply(summary.StaticIP)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *byocStaticIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan byocStaticIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocStaticIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byocStaticIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := ensureKey(state.DeleteIdempotencyKey.ValueString(), "tf-byoc-static-ip-delete", state.NodeID.ValueString())
	err := r.client.ReleaseBYOCStaticIP(ctx, state.NodeID.ValueString(), key)
	if err == nil {
		_, err = waitForStaticIP(ctx, r.client, state.NodeID.ValueString(), true)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to release BYOCloud static IP", err.Error())
	}
}

func (r *byocStaticIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("node_id"), req, resp)
}

func waitForStaticIP(ctx context.Context, api *client.Client, nodeID string, absent bool) (*client.BYOCStaticIP, error) {
	for {
		summary, err := api.GetBYOCNodeResources(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		address := summary.StaticIP
		if absent && staticIPAbsent(address) {
			return nil, nil
		}
		if !absent && address != nil && address.Assigned {
			return address, nil
		}
		if err := waitBYOCPoll(ctx); err != nil {
			return nil, err
		}
	}
}

func staticIPAbsent(address *client.BYOCStaticIP) bool {
	return address == nil || address.ProviderResourceID == "" || strings.EqualFold(strings.TrimSpace(address.Status), "deleted")
}

func (m *byocStaticIPModel) apply(address *client.BYOCStaticIP) {
	m.ID = types.StringValue(m.NodeID.ValueString())
	m.ProviderResourceID = optionalBYOCString(address.ProviderResourceID)
	m.IPAddress = optionalBYOCString(address.IPAddress)
	m.Status = optionalBYOCString(address.Status)
	m.Assigned = types.BoolValue(address.Assigned)
	m.LastSyncedAt = timeString(address.LastSyncedAt)
}
