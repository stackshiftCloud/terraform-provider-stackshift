package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*byocNodeResource)(nil)
var _ resource.ResourceWithConfigure = (*byocNodeResource)(nil)
var _ resource.ResourceWithImportState = (*byocNodeResource)(nil)

func NewBYOCNodeResource() resource.Resource { return &byocNodeResource{} }

type byocNodeResource struct{ client *client.Client }

type byocNodeModel struct {
	ID                   types.String `tfsdk:"id"`
	ProviderConnectionID types.String `tfsdk:"provider_connection_id"`
	Provider             types.String `tfsdk:"provider"`
	Region               types.String `tfsdk:"region"`
	TierSlug             types.String `tfsdk:"tier_slug"`
	Name                 types.String `tfsdk:"name"`
	IdempotencyKey       types.String `tfsdk:"idempotency_key"`
	DeleteIdempotencyKey types.String `tfsdk:"delete_idempotency_key"`
	OperationID          types.String `tfsdk:"operation_id"`
	OperationStatus      types.String `tfsdk:"operation_status"`
	OperationStep        types.String `tfsdk:"operation_step"`
	OperationProgress    types.Int64  `tfsdk:"operation_progress"`
	Status               types.String `tfsdk:"status"`
	BootstrapStatus      types.String `tfsdk:"bootstrap_status"`
	EnrollmentState      types.String `tfsdk:"enrollment_state"`
	ExternalResourceID   types.String `tfsdk:"external_resource_id"`
	OverlayIP            types.String `tfsdk:"overlay_ip"`
	PublicIP             types.String `tfsdk:"public_ip"`
	FailureReason        types.String `tfsdk:"failure_reason"`
	LastHeartbeatAt      types.String `tfsdk:"last_heartbeat_at"`
}

func (r *byocNodeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_node"
}

func (r *byocNodeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *byocNodeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{MarkdownDescription: "Provisions and safely deletes a durable BYOCloud node. Create and delete wait for their durable operations to reach a terminal provider-confirmed state.", Attributes: map[string]schema.Attribute{
		"id":                     schema.StringAttribute{Computed: true},
		"provider_connection_id": schema.StringAttribute{Required: true, PlanModifiers: replace},
		"provider":               schema.StringAttribute{Required: true, PlanModifiers: replace},
		"region":                 schema.StringAttribute{Required: true, PlanModifiers: replace},
		"tier_slug":              schema.StringAttribute{Required: true, PlanModifiers: replace},
		"name":                   schema.StringAttribute{Required: true, PlanModifiers: replace},
		"idempotency_key":        schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: replace},
		"delete_idempotency_key": schema.StringAttribute{Computed: true},
		"operation_id":           schema.StringAttribute{Computed: true},
		"operation_status":       schema.StringAttribute{Computed: true},
		"operation_step":         schema.StringAttribute{Computed: true},
		"operation_progress":     schema.Int64Attribute{Computed: true},
		"status":                 schema.StringAttribute{Computed: true},
		"bootstrap_status":       schema.StringAttribute{Computed: true},
		"enrollment_state":       schema.StringAttribute{Computed: true},
		"external_resource_id":   schema.StringAttribute{Computed: true},
		"overlay_ip":             schema.StringAttribute{Computed: true},
		"public_ip":              schema.StringAttribute{Computed: true},
		"failure_reason":         schema.StringAttribute{Computed: true},
		"last_heartbeat_at":      schema.StringAttribute{Computed: true},
	}}
}

func (r *byocNodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan byocNodeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := strings.TrimSpace(plan.IdempotencyKey.ValueString())
	if key == "" {
		key = deterministicIdempotencyKey("tf-byoc-node-create", plan.ProviderConnectionID.ValueString(), plan.Provider.ValueString(), plan.Region.ValueString(), plan.TierSlug.ValueString(), plan.Name.ValueString())
	}
	node, operation, err := r.client.ProvisionBYOCNode(ctx, map[string]any{
		"provider_connection_id": plan.ProviderConnectionID.ValueString(), "provider": plan.Provider.ValueString(),
		"region": plan.Region.ValueString(), "tier_slug": plan.TierSlug.ValueString(), "node_name": plan.Name.ValueString(),
	}, key)
	if err == nil {
		operation, err = r.client.WaitBYOCOperation(ctx, operation.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to provision BYOCloud node", err.Error())
		return
	}
	node, err = r.client.GetBYOCNode(ctx, node.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read provisioned BYOCloud node", err.Error())
		return
	}
	plan.IdempotencyKey = types.StringValue(key)
	deleteKey := deterministicIdempotencyKey("tf-byoc-node-delete", node.ID)
	plan.DeleteIdempotencyKey = types.StringValue(deleteKey)
	plan.applyNode(node)
	plan.applyOperation(operation)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocNodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byocNodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	node, err := r.client.GetBYOCNode(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read BYOCloud node", err.Error())
		return
	}
	state.applyNode(node)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *byocNodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan byocNodeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocNodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byocNodeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key := state.DeleteIdempotencyKey.ValueString()
	if key == "" {
		key = deterministicIdempotencyKey("tf-byoc-node-delete", state.ID.ValueString())
	}
	operation, err := r.client.DeleteBYOCNode(ctx, state.ID.ValueString(), key)
	if err == nil && operation != nil {
		_, err = r.client.WaitBYOCOperation(ctx, operation.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete BYOCloud node", err.Error())
	}
}

func (r *byocNodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *byocNodeModel) applyNode(node *client.BYOCNode) {
	m.ID = types.StringValue(node.ID)
	m.ProviderConnectionID = optionalBYOCString(node.ProviderConnectionID)
	m.Provider = types.StringValue(node.Provider)
	m.Region = types.StringValue(node.Region)
	m.TierSlug = optionalBYOCString(node.TierSlug)
	m.Name = types.StringValue(node.Name)
	m.Status = types.StringValue(node.Status)
	m.BootstrapStatus = optionalBYOCString(node.BootstrapStatus)
	m.EnrollmentState = optionalBYOCString(node.EnrollmentState)
	m.ExternalResourceID = optionalBYOCString(node.ExternalResourceID)
	m.OverlayIP = optionalBYOCString(node.OverlayIP)
	m.PublicIP = optionalBYOCString(node.PublicIP)
	m.FailureReason = optionalBYOCString(node.FailureReason)
	m.LastHeartbeatAt = timeString(node.LastHeartbeatAt)
}

func (m *byocNodeModel) applyOperation(operation *client.BYOCOperation) {
	if operation == nil {
		return
	}
	m.OperationID = types.StringValue(operation.ID)
	m.OperationStatus = types.StringValue(operation.Status)
	m.OperationStep = types.StringValue(operation.Step)
	m.OperationProgress = types.Int64Value(int64(operation.Progress))
}
