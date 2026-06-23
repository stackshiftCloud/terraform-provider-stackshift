package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*databaseCredentialsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*databaseCredentialsDataSource)(nil)

func NewDatabaseCredentialsDataSource() datasource.DataSource {
	return &databaseCredentialsDataSource{}
}

type databaseCredentialsDataSource struct{ client *client.Client }

type databaseCredentialsModel struct {
	DatabaseID       types.String `tfsdk:"database_id"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	DatabaseName     types.String `tfsdk:"database_name"`
	Host             types.String `tfsdk:"host"`
	Port             types.Int64  `tfsdk:"port"`
	TLSMode          types.String `tfsdk:"tls_mode"`
	ConnectionString types.String `tfsdk:"connection_string"`
}

func (d *databaseCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_credentials"
}

func (d *databaseCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *databaseCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"database_id":       schema.StringAttribute{Required: true},
			"username":          schema.StringAttribute{Computed: true, Sensitive: true},
			"password":          schema.StringAttribute{Computed: true, Sensitive: true},
			"database_name":     schema.StringAttribute{Computed: true, Sensitive: true},
			"host":              schema.StringAttribute{Computed: true, Sensitive: true},
			"port":              schema.Int64Attribute{Computed: true, Sensitive: true},
			"tls_mode":          schema.StringAttribute{Computed: true, Sensitive: true},
			"connection_string": schema.StringAttribute{Computed: true, Sensitive: true},
		},
	}
}

func (d *databaseCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state databaseCredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	creds, err := d.client.GetDatabaseCredentials(ctx, state.DatabaseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database credentials failed", err.Error())
		return
	}
	state.Username = types.StringValue(creds.Username)
	state.Password = types.StringValue(creds.Password)
	state.DatabaseName = types.StringValue(creds.DatabaseName)
	state.Host = types.StringValue(creds.Host)
	state.Port = types.Int64Value(int64(creds.Port))
	state.TLSMode = types.StringValue(creds.TLSMode)
	state.ConnectionString = types.StringValue(creds.ConnectionString)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
