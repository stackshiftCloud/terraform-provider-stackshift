package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*databaseDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*databaseDataSource)(nil)

func NewDatabaseDataSource() datasource.DataSource { return &databaseDataSource{} }

type databaseDataSource struct{ client *client.Client }

func (d *databaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database"
}

func (d *databaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *databaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Required: true},
			"project_id":     schema.StringAttribute{Computed: true},
			"name":           schema.StringAttribute{Computed: true},
			"type":           schema.StringAttribute{Computed: true},
			"version":        schema.StringAttribute{Computed: true},
			"size_gb":        schema.Int64Attribute{Computed: true},
			"target_node_id": schema.StringAttribute{Computed: true},
			"status":         schema.StringAttribute{Computed: true},
			"host":           schema.StringAttribute{Computed: true},
			"port":           schema.Int64Attribute{Computed: true},
			"tls_mode":       schema.StringAttribute{Computed: true},
			"database_name":  schema.StringAttribute{Computed: true},
			"created_at":     schema.StringAttribute{Computed: true},
		},
	}
}

func (d *databaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	db, err := d.client.GetDatabase(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database data source failed", err.Error())
		return
	}
	state.applyDatabase(db)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
