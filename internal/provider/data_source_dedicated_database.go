package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSourceWithConfigure = (*dedicatedDatabaseDataSource)(nil)

func NewDedicatedDatabaseDataSource() datasource.DataSource { return &dedicatedDatabaseDataSource{} }

type dedicatedDatabaseDataSource struct{ client *client.Client }

type dedicatedMemberModel struct {
	ID            types.String `tfsdk:"id"`
	Role          types.String `tfsdk:"role"`
	InstanceClass types.String `tfsdk:"instance_class"`
	Status        types.String `tfsdk:"status"`
	Site          types.String `tfsdk:"site"`
}

var dedicatedMemberType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id": types.StringType, "role": types.StringType, "instance_class": types.StringType, "status": types.StringType, "site": types.StringType,
}}

type dedicatedDatabaseDataModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Name                types.String `tfsdk:"name"`
	Engine              types.String `tfsdk:"engine"`
	Version             types.String `tfsdk:"version"`
	Status              types.String `tfsdk:"status"`
	InstanceClass       types.String `tfsdk:"instance_class"`
	AllocatedStorageGB  types.Int64  `tfsdk:"allocated_storage_gb"`
	StorageMaxGB        types.Int64  `tfsdk:"storage_max_gb"`
	BackupRetentionDays types.Int64  `tfsdk:"backup_retention_days"`
	DeletionProtection  types.Bool   `tfsdk:"deletion_protection"`
	MaintenanceWindow   types.Object `tfsdk:"maintenance_window"`
	Billing             types.String `tfsdk:"billing"`
	BillingStatus       types.String `tfsdk:"billing_status"`
	Standby             types.String `tfsdk:"standby"`
	OnStandbyLoss       types.String `tfsdk:"on_standby_loss"`
	MasterUsername      types.String `tfsdk:"master_username"`
	DatabaseName        types.String `tfsdk:"database_name"`
	Host                types.String `tfsdk:"host"`
	Port                types.Int64  `tfsdk:"port"`
	TLSMode             types.String `tfsdk:"tls_mode"`
	Endpoints           types.List   `tfsdk:"endpoints"`
	Members             types.List   `tfsdk:"members"`
}

func (d *dedicatedDatabaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database"
}

func (d *dedicatedDatabaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *dedicatedDatabaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a Dedicated database: its settings, status, connection addresses (including read replicas) and members.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.StringAttribute{Required: true},
			"project_id":            schema.StringAttribute{Computed: true},
			"name":                  schema.StringAttribute{Computed: true},
			"engine":                schema.StringAttribute{Computed: true, Description: "postgres or mysql."},
			"version":               schema.StringAttribute{Computed: true},
			"status":                schema.StringAttribute{Computed: true},
			"instance_class":        schema.StringAttribute{Computed: true},
			"allocated_storage_gb":  schema.Int64Attribute{Computed: true, Description: "The current storage size, including growth from autogrow."},
			"storage_max_gb":        schema.Int64Attribute{Computed: true},
			"backup_retention_days": schema.Int64Attribute{Computed: true},
			"deletion_protection":   schema.BoolAttribute{Computed: true},
			"maintenance_window": schema.SingleNestedAttribute{Computed: true, Attributes: map[string]schema.Attribute{
				"day_of_week":  schema.Int64Attribute{Computed: true},
				"start_minute": schema.Int64Attribute{Computed: true},
			}},
			"billing":         schema.StringAttribute{Computed: true},
			"billing_status":  schema.StringAttribute{Computed: true},
			"standby":         schema.StringAttribute{Computed: true},
			"on_standby_loss": schema.StringAttribute{Computed: true},
			"master_username": schema.StringAttribute{Computed: true},
			"database_name":   schema.StringAttribute{Computed: true},
			"host":            schema.StringAttribute{Computed: true},
			"port":            schema.Int64Attribute{Computed: true},
			"tls_mode":        schema.StringAttribute{Computed: true},
			"endpoints": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"protocol": schema.StringAttribute{Computed: true},
				"network":  schema.StringAttribute{Computed: true},
				"role":     schema.StringAttribute{Computed: true, Description: "Includes the read address once a replica has caught up."},
				"host":     schema.StringAttribute{Computed: true},
				"port":     schema.Int64Attribute{Computed: true},
				"tls_mode": schema.StringAttribute{Computed: true},
				"primary":  schema.BoolAttribute{Computed: true},
			}}},
			"members": schema.ListNestedAttribute{Computed: true, Description: "The primary, the standby and every read replica.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":             schema.StringAttribute{Computed: true},
					"role":           schema.StringAttribute{Computed: true, Description: "primary, standby or replica."},
					"instance_class": schema.StringAttribute{Computed: true},
					"status":         schema.StringAttribute{Computed: true},
					"site":           schema.StringAttribute{Computed: true},
				}}},
		},
	}
}

func (d *dedicatedDatabaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state dedicatedDatabaseDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	db, err := d.client.GetDedicatedDatabase(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift dedicated database data source failed", dedicatedErrorDetail(err))
		return
	}
	shared := dedicatedDatabaseModel{ID: types.StringValue(id), Endpoints: types.ListNull(dedicatedEndpointType)}
	shared.applyDatabase(ctx, db, &resp.Diagnostics)
	state.ProjectID, state.Name, state.Engine, state.Version = shared.ProjectID, shared.Name, shared.Engine, shared.Version
	state.Status, state.InstanceClass, state.AllocatedStorageGB, state.StorageMaxGB = shared.Status, shared.InstanceClass, shared.AllocatedStorageGB, shared.StorageMaxGB
	state.BackupRetentionDays, state.DeletionProtection, state.MaintenanceWindow = shared.BackupRetentionDays, shared.DeletionProtection, shared.MaintenanceWindow
	state.Billing, state.Standby, state.OnStandbyLoss = shared.Billing, shared.Standby, shared.OnStandbyLoss
	state.MasterUsername, state.DatabaseName, state.Host, state.Port, state.Endpoints = shared.MasterUsername, shared.DatabaseName, shared.Host, shared.Port, shared.Endpoints
	state.TLSMode = nonEmptyString(db.TLSMode)
	if state.MaintenanceWindow.IsNull() {
		state.MaintenanceWindow = types.ObjectNull(maintenanceWindowType.AttrTypes)
	}
	members := []dedicatedMemberModel{}
	ha, err := d.client.GetDedicatedHA(ctx, id)
	if err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Read StackShift dedicated database data source failed", dedicatedErrorDetail(err))
		return
	}
	if err == nil {
		shared.applyHA(ha, &resp.Diagnostics)
		state.InstanceClass = shared.InstanceClass
		state.Standby, state.OnStandbyLoss = types.StringValue(ha.StandbyMode), types.StringValue(ha.OnStandbyLoss)
		for _, member := range ha.Members {
			members = append(members, dedicatedMemberModel{ID: types.StringValue(member.ID), Role: types.StringValue(member.Role),
				InstanceClass: nonEmptyString(member.InstanceClass), Status: types.StringValue(member.Status), Site: nonEmptyString(member.Site)})
		}
	}
	list, diags := types.ListValueFrom(ctx, dedicatedMemberType, members)
	resp.Diagnostics.Append(diags...)
	state.Members = list
	state.BillingStatus = types.StringNull()
	if billing, err := d.client.GetDedicatedBilling(ctx, id); err == nil {
		state.BillingStatus = nonEmptyString(billing.Status)
		state.Billing = firstKnownString(billing.Mode, state.Billing)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
