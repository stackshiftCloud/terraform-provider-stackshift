package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSourceWithConfigure = (*databaseParameterDefinitionsDataSource)(nil)
var _ datasource.DataSourceWithValidateConfig = (*databaseParameterDefinitionsDataSource)(nil)

func NewDatabaseParameterDefinitionsDataSource() datasource.DataSource {
	return &databaseParameterDefinitionsDataSource{}
}

type databaseParameterDefinitionsDataSource struct{ client *client.Client }

type databaseParameterDefinitionModel struct {
	Name          types.String  `tfsdk:"name"`
	Kind          types.String  `tfsdk:"kind"`
	Min           types.Float64 `tfsdk:"min"`
	Max           types.Float64 `tfsdk:"max"`
	Values        types.List    `tfsdk:"values"`
	NeedsRestart  types.Bool    `tfsdk:"needs_restart"`
	EngineDefault types.String  `tfsdk:"engine_default"`
	Description   types.String  `tfsdk:"description"`
}

type databaseParameterDefinitionsModel struct {
	Family     types.String                       `tfsdk:"family"`
	Parameters []databaseParameterDefinitionModel `tfsdk:"parameters"`
}

func (d *databaseParameterDefinitionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_parameter_definitions"
}

func (d *databaseParameterDefinitionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *databaseParameterDefinitionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The engine settings you can put in a parameter group for one engine version, with their limits and whether changing them needs a restart.",
		Attributes: map[string]schema.Attribute{
			"family": schema.StringAttribute{Required: true, Description: "postgres15, postgres16, postgres17, postgres18 or mysql8.4."},
			"parameters": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"name":           schema.StringAttribute{Computed: true},
				"kind":           schema.StringAttribute{Computed: true, Description: "The type of value, such as integer, memory, duration or enum."},
				"min":            schema.Float64Attribute{Computed: true},
				"max":            schema.Float64Attribute{Computed: true},
				"values":         schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "The allowed values, for settings that take a fixed set."},
				"needs_restart":  schema.BoolAttribute{Computed: true},
				"engine_default": schema.StringAttribute{Computed: true},
				"description":    schema.StringAttribute{Computed: true},
			}}},
		},
	}
}

func (d *databaseParameterDefinitionsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var family types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("family"), &family)...)
	if !family.IsNull() && !family.IsUnknown() && !containsString(parameterGroupFamilies, family.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("family"), "Unsupported value", fmt.Sprintf("Use one of %v.", parameterGroupFamilies))
	}
}

func (d *databaseParameterDefinitionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseParameterDefinitionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	definitions, err := d.client.ListDedicatedParameterDefinitions(ctx, state.Family.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift parameter definitions failed", dedicatedErrorDetail(err))
		return
	}
	state.Parameters = []databaseParameterDefinitionModel{}
	for _, definition := range definitions {
		values, diags := types.ListValueFrom(ctx, types.StringType, definition.Values)
		resp.Diagnostics.Append(diags...)
		item := databaseParameterDefinitionModel{Name: types.StringValue(definition.Name), Kind: types.StringValue(definition.Kind),
			Min: types.Float64Null(), Max: types.Float64Null(), Values: values, NeedsRestart: types.BoolValue(definition.Restart),
			EngineDefault: nonEmptyString(definition.EngineValue), Description: types.StringValue(definition.Description)}
		if definition.Min != nil {
			item.Min = types.Float64Value(*definition.Min)
		}
		if definition.Max != nil {
			item.Max = types.Float64Value(*definition.Max)
		}
		state.Parameters = append(state.Parameters, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
