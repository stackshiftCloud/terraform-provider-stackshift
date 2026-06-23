package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*databaseResource)(nil)
var _ resource.ResourceWithConfigure = (*databaseResource)(nil)
var _ resource.ResourceWithImportState = (*databaseResource)(nil)

func NewDatabaseResource() resource.Resource { return &databaseResource{} }

type databaseResource struct{ client *client.Client }

type databaseModel struct {
	ID           types.String `tfsdk:"id"`
	ProjectID    types.String `tfsdk:"project_id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Version      types.String `tfsdk:"version"`
	SizeGB       types.Int64  `tfsdk:"size_gb"`
	TargetNodeID types.String `tfsdk:"target_node_id"`
	Status       types.String `tfsdk:"status"`
	Host         types.String `tfsdk:"host"`
	Port         types.Int64  `tfsdk:"port"`
	TLSMode      types.String `tfsdk:"tls_mode"`
	DatabaseName types.String `tfsdk:"database_name"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *databaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (r *databaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *databaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"project_id":     schema.StringAttribute{Required: true, PlanModifiers: replaceString},
			"name":           schema.StringAttribute{Required: true, PlanModifiers: replaceString},
			"type":           schema.StringAttribute{Required: true, PlanModifiers: replaceString},
			"version":        schema.StringAttribute{Required: true, PlanModifiers: replaceString},
			"size_gb":        schema.Int64Attribute{Required: true, PlanModifiers: replaceInt},
			"target_node_id": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}},
			"status":         schema.StringAttribute{Computed: true},
			"host":           schema.StringAttribute{Computed: true},
			"port":           schema.Int64Attribute{Computed: true},
			"tls_mode":       schema.StringAttribute{Computed: true},
			"database_name":  schema.StringAttribute{Computed: true},
			"created_at":     schema.StringAttribute{Computed: true},
		},
	}
}

func (r *databaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	db, err := r.client.CreateDatabase(ctx, plan.ProjectID.ValueString(), client.CreateDatabaseRequest{
		Name:         plan.Name.ValueString(),
		Type:         plan.Type.ValueString(),
		Version:      plan.Version.ValueString(),
		SizeGB:       int(plan.SizeGB.ValueInt64()),
		TargetNodeID: stringPtr(plan.TargetNodeID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift database failed", err.Error())
		return
	}
	plan.applyDatabase(db)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	db, err := r.client.GetDatabase(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database failed", err.Error())
		return
	}
	state.applyDatabase(db)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDatabase(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift database failed", err.Error())
	}
}

func (r *databaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *databaseModel) applyDatabase(db *client.Database) {
	m.ID = types.StringValue(db.ID)
	if db.ProjectID != nil {
		m.ProjectID = types.StringValue(*db.ProjectID)
	}
	m.Name = types.StringValue(db.Name)
	m.Type = types.StringValue(db.Type)
	m.Version = types.StringValue(db.Version)
	m.SizeGB = types.Int64Value(int64(db.SizeGB))
	m.TargetNodeID = stringValue(db.TargetNodeID)
	m.Status = types.StringValue(db.Status)
	m.Host = types.StringValue(db.Host)
	m.Port = types.Int64Value(int64(db.Port))
	m.TLSMode = types.StringValue(db.TLSMode)
	m.DatabaseName = types.StringValue(db.DatabaseName)
	m.CreatedAt = timeString(db.CreatedAt)
}
