package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*assetAnalyticsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*assetAnalyticsDataSource)(nil)

func NewAssetAnalyticsDataSource() datasource.DataSource { return &assetAnalyticsDataSource{} }

type assetAnalyticsDataSource struct{ client *client.Client }

type assetAnalyticsModel struct {
	ID                      types.String `tfsdk:"id"`
	TotalAssets             types.Int64  `tfsdk:"total_assets"`
	TotalBytes              types.Int64  `tfsdk:"total_bytes"`
	PeriodIngressBytes      types.Int64  `tfsdk:"period_ingress_bytes"`
	PeriodEgressBytes       types.Int64  `tfsdk:"period_egress_bytes"`
	PeriodOriginBytes       types.Int64  `tfsdk:"period_origin_bytes"`
	PeriodVerificationBytes types.Int64  `tfsdk:"period_verification_bytes"`
	PeriodTransformCount    types.Int64  `tfsdk:"period_transform_count"`
	PeriodAIRequests        types.Int64  `tfsdk:"period_ai_requests"`
	PeriodLogicalByteHours  types.Int64  `tfsdk:"period_logical_storage_byte_hours"`
	PeriodPhysicalByteHours types.Int64  `tfsdk:"period_physical_storage_byte_hours"`
	PeriodDerivedByteHours  types.Int64  `tfsdk:"period_derived_storage_byte_hours"`
	PeriodTransformMillis   types.Int64  `tfsdk:"period_transform_compute_ms"`
	PeriodVideoInputSeconds types.Int64  `tfsdk:"period_video_input_seconds"`
	PeriodAICostMicros      types.Int64  `tfsdk:"period_ai_cost_micros"`
	ByBucket                types.Map    `tfsdk:"by_bucket"`
	ByMimeFamily            types.Map    `tfsdk:"by_mime_family"`
	ByVisibility            types.Map    `tfsdk:"by_visibility"`
}

func (d *assetAnalyticsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_analytics"
}

func (d *assetAnalyticsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *assetAnalyticsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := map[string]schema.Attribute{"id": schema.StringAttribute{Computed: true}}
	for _, name := range []string{"total_assets", "total_bytes", "period_ingress_bytes", "period_egress_bytes", "period_origin_bytes", "period_verification_bytes", "period_transform_count", "period_ai_requests", "period_logical_storage_byte_hours", "period_physical_storage_byte_hours", "period_derived_storage_byte_hours", "period_transform_compute_ms", "period_video_input_seconds", "period_ai_cost_micros"} {
		attributes[name] = schema.Int64Attribute{Computed: true}
	}
	for _, name := range []string{"by_bucket", "by_mime_family", "by_visibility"} {
		attributes[name] = schema.MapAttribute{Computed: true, ElementType: types.Int64Type}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads the current append-only usage projection for StackShift Assets.", Attributes: attributes}
}

func (d *assetAnalyticsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	summary, err := d.client.GetAssetAnalytics(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets analytics", err.Error())
		return
	}
	byBucket, d1 := types.MapValueFrom(ctx, types.Int64Type, summary.ByBucket)
	byMime, d2 := types.MapValueFrom(ctx, types.Int64Type, summary.ByMimeFamily)
	byVisibility, d3 := types.MapValueFrom(ctx, types.Int64Type, summary.ByVisibility)
	resp.Diagnostics.Append(d1...)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(d3...)
	state := assetAnalyticsModel{
		ID: types.StringValue("current"), TotalAssets: types.Int64Value(summary.TotalAssets), TotalBytes: types.Int64Value(summary.TotalBytes),
		PeriodIngressBytes: types.Int64Value(summary.PeriodIngressBytes), PeriodEgressBytes: types.Int64Value(summary.PeriodEgressBytes),
		PeriodOriginBytes: types.Int64Value(summary.PeriodOriginBytes), PeriodVerificationBytes: types.Int64Value(summary.PeriodVerificationBytes),
		PeriodTransformCount: types.Int64Value(summary.PeriodTransformCount), PeriodAIRequests: types.Int64Value(summary.PeriodAIRequests),
		PeriodLogicalByteHours: types.Int64Value(summary.PeriodLogicalByteHours), PeriodPhysicalByteHours: types.Int64Value(summary.PeriodPhysicalByteHours),
		PeriodDerivedByteHours: types.Int64Value(summary.PeriodDerivedByteHours), PeriodTransformMillis: types.Int64Value(summary.PeriodTransformMillis),
		PeriodVideoInputSeconds: types.Int64Value(summary.PeriodVideoInputSeconds), PeriodAICostMicros: types.Int64Value(summary.PeriodAICostMicros),
		ByBucket: byBucket, ByMimeFamily: byMime, ByVisibility: byVisibility,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
