package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSourceWithConfigure = (*regionsDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*appSizesDataSource)(nil)

func NewRegionsDataSource() datasource.DataSource  { return &regionsDataSource{} }
func NewAppSizesDataSource() datasource.DataSource { return &appSizesDataSource{} }

type regionsDataSource struct{ client *client.Client }
type appSizesDataSource struct{ client *client.Client }

type regionModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	City               types.String `tfsdk:"city"`
	Country            types.String `tfsdk:"country"`
	CountryCode        types.String `tfsdk:"country_code"`
	Default            types.Bool   `tfsdk:"default"`
	Status             types.String `tfsdk:"status"`
	AppsAvailable      types.Bool   `tfsdk:"apps_available"`
	AppTiers           []string     `tfsdk:"app_tiers"`
	DatabasesAvailable types.Bool   `tfsdk:"databases_available"`
	DatabaseTiers      []string     `tfsdk:"database_tiers"`
	VPSAvailable       types.Bool   `tfsdk:"vps_available"`
}

type regionsModel struct {
	Product       types.String  `tfsdk:"product"`
	DefaultRegion types.String  `tfsdk:"default_region"`
	Regions       []regionModel `tfsdk:"regions"`
}

type appSizeModel struct {
	Slug            types.String `tfsdk:"slug"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	MemoryMB        types.Int64  `tfsdk:"memory_mb"`
	CPUMillicores   types.Int64  `tfsdk:"cpu_millicores"`
	Listed          types.Bool   `tfsdk:"listed"`
	Recommended     types.Bool   `tfsdk:"recommended"`
	PriceNGNMonthly types.Int64  `tfsdk:"price_ngn_monthly"`
	IncludedInPlan  types.Bool   `tfsdk:"included_in_plan"`
	MinPlan         types.String `tfsdk:"min_plan"`
	Plans           []string     `tfsdk:"plans"`
}

type appSizePlanModel struct {
	Slug             types.String `tfsdk:"slug"`
	Name             types.String `tfsdk:"name"`
	MaxMemoryMB      types.Int64  `tfsdk:"max_memory_mb"`
	MaxCPUMillicores types.Int64  `tfsdk:"max_cpu_millicores"`
	AppTier          types.String `tfsdk:"app_tier"`
	DefaultSize      types.String `tfsdk:"default_size"`
}

type appSizesModel struct {
	ListedOnly types.Bool         `tfsdk:"listed_only"`
	Version    types.Int64        `tfsdk:"version"`
	Currency   types.String       `tfsdk:"currency"`
	Sizes      []appSizeModel     `tfsdk:"sizes"`
	Plans      []appSizePlanModel `tfsdk:"plans"`
}

func (d *regionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_regions"
}

func (d *regionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *regionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Regions where StackShift can run apps, databases and servers right now.",
		Attributes: map[string]schema.Attribute{
			"product": schema.StringAttribute{
				Optional:    true,
				Description: "Only return regions where this product is available: apps, databases or vps.",
			},
			"default_region": schema.StringAttribute{Computed: true},
			"regions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                  schema.StringAttribute{Computed: true},
					"name":                schema.StringAttribute{Computed: true},
					"city":                schema.StringAttribute{Computed: true},
					"country":             schema.StringAttribute{Computed: true},
					"country_code":        schema.StringAttribute{Computed: true},
					"default":             schema.BoolAttribute{Computed: true},
					"status":              schema.StringAttribute{Computed: true},
					"apps_available":      schema.BoolAttribute{Computed: true},
					"app_tiers":           schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"databases_available": schema.BoolAttribute{Computed: true},
					"database_tiers":      schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"vps_available":       schema.BoolAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state regionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	product := state.Product.ValueString()
	if product != "" && product != "apps" && product != "databases" && product != "vps" {
		resp.Diagnostics.AddAttributeError(path.Root("product"), "Unknown product", "Use apps, databases or vps.")
		return
	}
	catalog, err := d.client.GetRegions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift regions failed", err.Error())
		return
	}
	state.DefaultRegion = types.StringValue(catalog.DefaultRegion)
	state.Regions = regionModels(catalog.Regions, product)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func regionModels(regions []client.Region, product string) []regionModel {
	models := []regionModel{}
	for _, region := range regions {
		available := map[string]bool{"": true, "apps": region.Products.Apps.Available, "databases": region.Products.Databases.Available, "vps": region.Products.VPS.Available}
		if !available[product] {
			continue
		}
		models = append(models, regionModel{
			ID: types.StringValue(region.ID), Name: types.StringValue(region.Name), City: types.StringValue(region.City),
			Country: types.StringValue(region.Country), CountryCode: types.StringValue(region.CountryCode),
			Default: types.BoolValue(region.Default), Status: types.StringValue(region.Status),
			AppsAvailable: types.BoolValue(region.Products.Apps.Available), AppTiers: nonNil(region.Products.Apps.Tiers),
			DatabasesAvailable: types.BoolValue(region.Products.Databases.Available), DatabaseTiers: nonNil(region.Products.Databases.Tiers),
			VPSAvailable: types.BoolValue(region.Products.VPS.Available),
		})
	}
	return models
}

func (d *appSizesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_sizes"
}

func (d *appSizesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *appSizesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "App sizes and each plan's limits. The project resource has no size attribute yet; set a size with the CLI or API resource_preset.",
		Attributes: map[string]schema.Attribute{
			"listed_only": schema.BoolAttribute{Optional: true, Description: "Only return sizes shown in the size picker."},
			"version":     schema.Int64Attribute{Computed: true},
			"currency":    schema.StringAttribute{Computed: true},
			"sizes": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"slug":              schema.StringAttribute{Computed: true},
					"name":              schema.StringAttribute{Computed: true},
					"description":       schema.StringAttribute{Computed: true},
					"memory_mb":         schema.Int64Attribute{Computed: true},
					"cpu_millicores":    schema.Int64Attribute{Computed: true},
					"listed":            schema.BoolAttribute{Computed: true},
					"recommended":       schema.BoolAttribute{Computed: true},
					"price_ngn_monthly": schema.Int64Attribute{Computed: true, Description: "Null when the size is included in the plan."},
					"included_in_plan":  schema.BoolAttribute{Computed: true},
					"min_plan":          schema.StringAttribute{Computed: true},
					"plans":             schema.ListAttribute{Computed: true, ElementType: types.StringType},
				}},
			},
			"plans": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"slug":               schema.StringAttribute{Computed: true},
					"name":               schema.StringAttribute{Computed: true},
					"max_memory_mb":      schema.Int64Attribute{Computed: true},
					"max_cpu_millicores": schema.Int64Attribute{Computed: true, Description: "-1 means unlimited."},
					"app_tier":           schema.StringAttribute{Computed: true},
					"default_size":       schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *appSizesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state appSizesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	catalog, err := d.client.GetAppSizes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift app sizes failed", err.Error())
		return
	}
	state.Version = types.Int64Value(catalog.Version)
	state.Currency = types.StringValue(catalog.Currency)
	state.Sizes = []appSizeModel{}
	for _, size := range catalog.Sizes {
		if state.ListedOnly.ValueBool() && !size.Listed {
			continue
		}
		state.Sizes = append(state.Sizes, appSizeModel{
			Slug: types.StringValue(size.Slug), Name: types.StringValue(size.Name), Description: types.StringValue(size.Description),
			MemoryMB: types.Int64Value(size.MemoryMB), CPUMillicores: types.Int64Value(size.CPUMillicores),
			Listed: types.BoolValue(size.Listed), Recommended: types.BoolValue(size.Recommended),
			PriceNGNMonthly: types.Int64PointerValue(size.PriceNGNMonthly), IncludedInPlan: types.BoolValue(size.IncludedInPlan),
			MinPlan: types.StringValue(size.MinPlan), Plans: nonNil(size.Plans),
		})
	}
	state.Plans = []appSizePlanModel{}
	for _, plan := range catalog.Plans {
		state.Plans = append(state.Plans, appSizePlanModel{
			Slug: types.StringValue(plan.Slug), Name: types.StringValue(plan.Name),
			MaxMemoryMB: types.Int64Value(plan.MaxMemoryMB), MaxCPUMillicores: types.Int64Value(plan.MaxCPUMillicores),
			AppTier: types.StringValue(plan.AppTier), DefaultSize: types.StringValue(plan.DefaultSize),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
