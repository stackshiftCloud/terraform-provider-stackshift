package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSourceWithConfigure = (*databaseInstanceClassesDataSource)(nil)

func NewDatabaseInstanceClassesDataSource() datasource.DataSource {
	return &databaseInstanceClassesDataSource{}
}

type databaseInstanceClassesDataSource struct{ client *client.Client }

type databaseInstanceClassModel struct {
	Slug                     types.String `tfsdk:"slug"`
	VCPU                     types.Int64  `tfsdk:"vcpu"`
	MemoryMB                 types.Int64  `tfsdk:"memory_mb"`
	CPUPolicy                types.String `tfsdk:"cpu_policy"`
	MaxConnections           types.Int64  `tfsdk:"max_connections"`
	PooledMaxConnections     types.Int64  `tfsdk:"pooled_max_connections"`
	DiskIOPS                 types.Int64  `tfsdk:"disk_iops"`
	DiskBytesPerSec          types.Int64  `tfsdk:"disk_bytes_per_sec"`
	NetworkKBps              types.Int64  `tfsdk:"network_kbps"`
	Active                   types.Bool   `tfsdk:"active"`
	Currency                 types.String `tfsdk:"currency"`
	MonthlyPriceMinor        types.Int64  `tfsdk:"monthly_price_minor"`
	StorageGBMonthPriceMinor types.Int64  `tfsdk:"storage_gb_month_price_minor"`
	BackupGBMonthPriceMinor  types.Int64  `tfsdk:"backup_gb_month_price_minor"`
}

type databaseInstanceClassesModel struct {
	Classes []databaseInstanceClassModel `tfsdk:"classes"`
}

func (d *databaseInstanceClassesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_instance_classes"
}

func (d *databaseInstanceClassesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *databaseInstanceClassesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sizes you can pick for a Dedicated database. Dedicated must be switched on for your account.",
		Attributes: map[string]schema.Attribute{
			"classes": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"slug":                         schema.StringAttribute{Computed: true},
					"vcpu":                         schema.Int64Attribute{Computed: true},
					"memory_mb":                    schema.Int64Attribute{Computed: true},
					"cpu_policy":                   schema.StringAttribute{Computed: true, Description: "shared or dedicated."},
					"max_connections":              schema.Int64Attribute{Computed: true},
					"pooled_max_connections":       schema.Int64Attribute{Computed: true},
					"disk_iops":                    schema.Int64Attribute{Computed: true},
					"disk_bytes_per_sec":           schema.Int64Attribute{Computed: true},
					"network_kbps":                 schema.Int64Attribute{Computed: true},
					"active":                       schema.BoolAttribute{Computed: true},
					"currency":                     schema.StringAttribute{Computed: true},
					"monthly_price_minor":          schema.Int64Attribute{Computed: true, Description: "In the currency's smallest unit."},
					"storage_gb_month_price_minor": schema.Int64Attribute{Computed: true, Description: "In the currency's smallest unit."},
					"backup_gb_month_price_minor":  schema.Int64Attribute{Computed: true, Description: "In the currency's smallest unit."},
				}},
			},
		},
	}
}

func (d *databaseInstanceClassesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	classes, err := d.client.ListDatabaseInstanceClasses(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database instance classes failed", err.Error())
		return
	}
	state := databaseInstanceClassesModel{Classes: []databaseInstanceClassModel{}}
	for _, class := range classes {
		state.Classes = append(state.Classes, databaseInstanceClassModel{
			Slug: types.StringValue(class.Slug), VCPU: types.Int64Value(class.VCPU), MemoryMB: types.Int64Value(class.MemoryMB),
			CPUPolicy: types.StringValue(class.CPUPolicy), MaxConnections: types.Int64Value(class.MaxConnections),
			PooledMaxConnections: types.Int64Value(class.PooledMaxConnections), DiskIOPS: types.Int64Value(class.DiskIOPS),
			DiskBytesPerSec: types.Int64Value(class.DiskBytesPerSec), NetworkKBps: types.Int64Value(class.NetworkKBps),
			Active: types.BoolValue(class.IsActive), Currency: types.StringValue(class.Revision.Currency),
			MonthlyPriceMinor:        types.Int64Value(class.Revision.MonthlyPriceMinor),
			StorageGBMonthPriceMinor: types.Int64Value(class.Revision.StorageGBMonthPriceMinor),
			BackupGBMonthPriceMinor:  types.Int64Value(class.Revision.BackupGBMonthPriceMinor),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
