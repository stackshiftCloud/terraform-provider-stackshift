package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*assetDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*assetDataSource)(nil)

func NewAssetDataSource() datasource.DataSource { return &assetDataSource{} }

type assetDataSource struct{ client *client.Client }

type assetDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Bucket            types.String `tfsdk:"bucket"`
	Key               types.String `tfsdk:"key"`
	OriginalName      types.String `tfsdk:"original_name"`
	MimeType          types.String `tfsdk:"mime_type"`
	Size              types.Int64  `tfsdk:"size"`
	ChecksumSHA256    types.String `tfsdk:"checksum_sha256"`
	Visibility        types.String `tfsdk:"visibility"`
	Status            types.String `tfsdk:"status"`
	ReplicationStatus types.String `tfsdk:"replication_status"`
	Generation        types.Int64  `tfsdk:"generation"`
	Revision          types.Int64  `tfsdk:"revision"`
	URL               types.String `tfsdk:"url"`
}

func (d *assetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset"
}

func (d *assetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *assetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads immutable identity, security state, and placement status for a StackShift Asset.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "bucket": schema.StringAttribute{Computed: true},
		"key": schema.StringAttribute{Computed: true}, "original_name": schema.StringAttribute{Computed: true},
		"mime_type": schema.StringAttribute{Computed: true}, "size": schema.Int64Attribute{Computed: true},
		"checksum_sha256": schema.StringAttribute{Computed: true}, "visibility": schema.StringAttribute{Computed: true},
		"status": schema.StringAttribute{Computed: true}, "replication_status": schema.StringAttribute{Computed: true},
		"generation": schema.Int64Attribute{Computed: true}, "revision": schema.Int64Attribute{Computed: true},
		"url": schema.StringAttribute{Computed: true},
	}}
}

func (d *assetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config assetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	asset, err := d.client.GetAsset(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read StackShift Asset", err.Error())
		return
	}
	state := assetDataSourceModel{
		ID: types.StringValue(asset.ID), Bucket: types.StringValue(asset.Bucket), Key: types.StringValue(asset.Key),
		OriginalName: types.StringValue(asset.OriginalName), MimeType: types.StringValue(asset.MimeType), Size: types.Int64Value(asset.Size),
		ChecksumSHA256: types.StringValue(asset.ChecksumSHA256), Visibility: types.StringValue(asset.Visibility), Status: types.StringValue(asset.Status),
		ReplicationStatus: types.StringValue(asset.ReplicationStatus), Generation: types.Int64Value(asset.Generation), Revision: types.Int64Value(asset.Revision),
	}
	if asset.URL == "" {
		state.URL = types.StringNull()
	} else {
		state.URL = types.StringValue(asset.URL)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
