package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*assetJobDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*assetJobDataSource)(nil)

func NewAssetJobDataSource() datasource.DataSource { return &assetJobDataSource{} }

type assetJobDataSource struct{ client *client.Client }

type assetJobDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	JobName         types.String `tfsdk:"job_name"`
	Status          types.String `tfsdk:"status"`
	ResultJSON      types.String `tfsdk:"result_json"`
	ErrorJSON       types.String `tfsdk:"error_json"`
	Attempts        types.Int64  `tfsdk:"attempts"`
	MaxAttempts     types.Int64  `tfsdk:"max_attempts"`
	ProgressPercent types.Int64  `tfsdk:"progress_percent"`
	Phase           types.String `tfsdk:"phase"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
}

func (d *assetJobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_job"
}

func (d *assetJobDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *assetJobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads durable processing status and progress for a StackShift Assets job.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "job_name": schema.StringAttribute{Computed: true},
		"status": schema.StringAttribute{Computed: true}, "result_json": schema.StringAttribute{Computed: true},
		"error_json": schema.StringAttribute{Computed: true}, "attempts": schema.Int64Attribute{Computed: true},
		"max_attempts": schema.Int64Attribute{Computed: true}, "progress_percent": schema.Int64Attribute{Computed: true},
		"phase": schema.StringAttribute{Computed: true}, "created_at": schema.StringAttribute{Computed: true},
		"updated_at": schema.StringAttribute{Computed: true},
	}}
}

func (d *assetJobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config assetJobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	job, err := d.client.GetAssetJob(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets job", err.Error())
		return
	}
	state := assetJobDataSourceModel{
		ID: types.StringValue(job.ID), JobName: types.StringValue(job.JobName), Status: types.StringValue(job.Status),
		Attempts: types.Int64Value(job.Attempts), MaxAttempts: types.Int64Value(job.MaxAttempts), Phase: types.StringValue(job.Phase),
		CreatedAt: types.StringValue(job.CreatedAt.UTC().Format(time.RFC3339)), UpdatedAt: types.StringValue(job.UpdatedAt.UTC().Format(time.RFC3339)),
	}
	state.ResultJSON = optionalAssetRawJSON(job.Result)
	state.ErrorJSON = optionalAssetRawJSON(job.Error)
	if job.ProgressPercent == nil {
		state.ProgressPercent = types.Int64Null()
	} else {
		state.ProgressPercent = types.Int64Value(*job.ProgressPercent)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func optionalAssetRawJSON(value []byte) types.String {
	if len(value) == 0 || string(value) == "null" {
		return types.StringNull()
	}
	return types.StringValue(string(value))
}
