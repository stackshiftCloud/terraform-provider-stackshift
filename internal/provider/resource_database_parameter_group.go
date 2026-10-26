package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*databaseParameterGroupResource)(nil)
var _ resource.ResourceWithImportState = (*databaseParameterGroupResource)(nil)
var _ resource.ResourceWithValidateConfig = (*databaseParameterGroupResource)(nil)

var parameterGroupFamilies = []string{"postgres15", "postgres16", "postgres17", "postgres18", "mysql8.4"}

func NewDatabaseParameterGroupResource() resource.Resource { return &databaseParameterGroupResource{} }

type databaseParameterGroupResource struct{ client *client.Client }

type databaseParameterGroupModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Family      types.String `tfsdk:"family"`
	Description types.String `tfsdk:"description"`
	Parameters  types.Map    `tfsdk:"parameters"`
	Version     types.Int64  `tfsdk:"version"`
	Databases   types.Int64  `tfsdk:"databases"`
}

func (r *databaseParameterGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_parameter_group"
}

func (r *databaseParameterGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *databaseParameterGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A named set of database engine settings that you can attach to Dedicated databases of the same engine and version. Dedicated must be switched on for your account.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, Description: "1 to 60 characters, unique in your account. Changes in place."},
			"family": schema.StringAttribute{Required: true, PlanModifiers: replaceString(),
				Description: "The engine and version the group is for: postgres15, postgres16, postgres17, postgres18 or mysql8.4. Changing this replaces the group."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Changes in place."},
			"parameters": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				Default:     mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				Description: "Setting names and values. See the stackshift_database_parameter_definitions data source for what each family accepts. Changes in place."},
			"version":   schema.Int64Attribute{Computed: true, Description: "Goes up each time the group changes."},
			"databases": schema.Int64Attribute{Computed: true, Description: "How many databases use this group."},
		},
	}
}

func (r *databaseParameterGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var family types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("family"), &family)...)
	if !family.IsNull() && !family.IsUnknown() && !containsString(parameterGroupFamilies, family.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("family"), "Unsupported value", fmt.Sprintf("Use one of %v.", parameterGroupFamilies))
	}
}

func (r *databaseParameterGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseParameterGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	group, err := r.client.CreateDedicatedParameterGroup(ctx, plan.Name.ValueString(), plan.Family.ValueString(), plan.Description.ValueString(),
		mapFromTerraform(ctx, plan.Parameters, &resp.Diagnostics))
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift parameter group failed", dedicatedErrorDetail(err))
		return
	}
	plan.ID, plan.Version, plan.Databases = types.StringValue(group.ID), types.Int64Value(group.Version), types.Int64Value(group.Databases)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseParameterGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseParameterGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	group, err := r.client.GetDedicatedParameterGroup(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift parameter group failed", dedicatedErrorDetail(err))
		return
	}
	state.Name, state.Family, state.Description = types.StringValue(group.Name), types.StringValue(group.Family), types.StringValue(group.Description)
	state.Version, state.Databases = types.Int64Value(group.Version), types.Int64Value(group.Databases)
	if group.Parameters == nil {
		group.Parameters = map[string]string{}
	}
	state.Parameters = terraformMap(ctx, group.Parameters, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseParameterGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan databaseParameterGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	group, err := r.client.UpdateDedicatedParameterGroup(ctx, plan.ID.ValueString(), plan.Name.ValueString(), plan.Description.ValueString(),
		mapFromTerraform(ctx, plan.Parameters, &resp.Diagnostics))
	if err != nil {
		resp.Diagnostics.AddError("Update StackShift parameter group failed", dedicatedErrorDetail(err))
		return
	}
	plan.Version, plan.Databases = types.Int64Value(group.Version), types.Int64Value(group.Databases)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseParameterGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseParameterGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDedicatedParameterGroup(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift parameter group failed", dedicatedErrorDetail(err))
	}
}

func (r *databaseParameterGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
