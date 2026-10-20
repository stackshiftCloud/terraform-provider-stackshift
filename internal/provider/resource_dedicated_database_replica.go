package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseReplicaResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseReplicaResource)(nil)

func NewDedicatedDatabaseReplicaResource() resource.Resource {
	return &dedicatedDatabaseReplicaResource{}
}

type dedicatedDatabaseReplicaResource struct{ client *client.Client }

type dedicatedDatabaseReplicaModel struct {
	ID                 types.String `tfsdk:"id"`
	DatabaseID         types.String `tfsdk:"database_id"`
	InstanceClass      types.String `tfsdk:"instance_class"`
	AcceptPriceChanges types.Bool   `tfsdk:"accept_price_changes"`
	Status             types.String `tfsdk:"status"`
	Site               types.String `tfsdk:"site"`
}

func (r *dedicatedDatabaseReplicaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_replica"
}

func (r *dedicatedDatabaseReplicaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedDatabaseReplicaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A read replica of a Dedicated database. Up to 5 per database. Prices: https://stackshift.cloud/pricing.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"instance_class": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
				Description:   "Defaults to the database's size. Needs at least half the database's memory."},
			"accept_price_changes": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Allow the prorated charge for the replica to be taken from the wallet."},
			"status": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"site":   schema.StringAttribute{Computed: true, PlanModifiers: keep},
		},
	}
}

func (r *dedicatedDatabaseReplicaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseReplicaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	databaseID := plan.DatabaseID.ValueString()
	var replica *client.DedicatedReplica
	if !paidChange(&resp.Diagnostics, "read replica", plan.AcceptPriceChanges.ValueBool(), func(confirm *int64) error {
		var err error
		replica, err = r.client.AddDedicatedReplica(ctx, databaseID, plan.InstanceClass.ValueString(), confirm)
		return err
	}) {
		return
	}
	plan.ID = types.StringValue(replica.ID)
	plan.InstanceClass = firstKnownString(replica.InstanceClass, plan.InstanceClass)
	if plan.InstanceClass.IsUnknown() {
		plan.InstanceClass = types.StringNull()
	}
	plan.Status, plan.Site = types.StringNull(), types.StringNull()
	if ha, err := r.client.GetDedicatedHA(ctx, databaseID); err == nil {
		plan.applyMember(ha)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseReplicaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseReplicaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ha, err := r.client.GetDedicatedHA(ctx, state.DatabaseID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database replica failed", dedicatedErrorDetail(err))
		return
	}
	if !state.applyMember(ha) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseReplicaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dedicatedDatabaseReplicaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseReplicaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabaseReplicaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RemoveDedicatedReplica(ctx, state.DatabaseID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Remove StackShift database replica failed", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseReplicaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Use database_id:replica_id. "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *dedicatedDatabaseReplicaModel) applyMember(ha *client.DedicatedHA) bool {
	for _, member := range ha.Members {
		if member.Role != "replica" || member.ID != m.ID.ValueString() {
			continue
		}
		m.InstanceClass = firstKnownString(member.InstanceClass, m.InstanceClass)
		m.Status = nonEmptyString(member.Status)
		m.Site = nonEmptyString(member.Site)
		return true
	}
	return false
}
