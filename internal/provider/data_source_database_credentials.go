package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	TLSServerName    types.String `tfsdk:"tls_server_name"`
	CACertificate    types.String `tfsdk:"ca_certificate"`
	Endpoints        types.List   `tfsdk:"endpoints"`
}

type databaseConnectionEndpointModel struct {
	Protocol         types.String `tfsdk:"protocol"`
	Network          types.String `tfsdk:"network"`
	Role             types.String `tfsdk:"role"`
	Primary          types.Bool   `tfsdk:"primary"`
	Host             types.String `tfsdk:"host"`
	Port             types.Int64  `tfsdk:"port"`
	TLSMode          types.String `tfsdk:"tls_mode"`
	TLSServerName    types.String `tfsdk:"tls_server_name"`
	ConnectionString types.String `tfsdk:"connection_string"`
	CAFilename       types.String `tfsdk:"ca_filename"`
}

var databaseConnectionEndpointType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"protocol": types.StringType, "network": types.StringType, "role": types.StringType, "primary": types.BoolType,
	"host": types.StringType, "port": types.Int64Type, "tls_mode": types.StringType, "tls_server_name": types.StringType,
	"connection_string": types.StringType, "ca_filename": types.StringType,
}}

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
			"tls_server_name":   schema.StringAttribute{Computed: true, Description: "The name to check the server certificate against."},
			"ca_certificate":    schema.StringAttribute{Computed: true, Description: "The certificate authority to trust when connecting."},
			"endpoints": schema.ListNestedAttribute{
				Computed: true, Sensitive: true,
				Description: "Every address the database answers on, including read replicas, each with its own connection string.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"protocol":          schema.StringAttribute{Computed: true},
					"network":           schema.StringAttribute{Computed: true},
					"role":              schema.StringAttribute{Computed: true},
					"primary":           schema.BoolAttribute{Computed: true},
					"host":              schema.StringAttribute{Computed: true},
					"port":              schema.Int64Attribute{Computed: true},
					"tls_mode":          schema.StringAttribute{Computed: true},
					"tls_server_name":   schema.StringAttribute{Computed: true},
					"connection_string": schema.StringAttribute{Computed: true},
					"ca_filename":       schema.StringAttribute{Computed: true},
				}},
			},
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
	state.TLSServerName = nonEmptyString(creds.TLSServerName)
	state.CACertificate = nonEmptyString(creds.CACertificate)
	endpoints := []databaseConnectionEndpointModel{}
	for _, endpoint := range creds.Endpoints {
		endpoints = append(endpoints, databaseConnectionEndpointModel{
			Protocol: types.StringValue(endpoint.Protocol), Network: types.StringValue(endpoint.Network), Role: types.StringValue(endpoint.Role),
			Primary: types.BoolValue(endpoint.Primary), Host: types.StringValue(endpoint.Host), Port: types.Int64Value(int64(endpoint.Port)),
			TLSMode: types.StringValue(endpoint.TLSMode), TLSServerName: nonEmptyString(endpoint.TLSServerName),
			ConnectionString: types.StringValue(endpoint.ConnectionString), CAFilename: nonEmptyString(endpoint.CAFilename),
		})
	}
	list, diags := types.ListValueFrom(ctx, databaseConnectionEndpointType, endpoints)
	resp.Diagnostics.Append(diags...)
	state.Endpoints = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
