package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSourceWithConfigure = (*dedicatedDatabaseBackupsDataSource)(nil)

func NewDedicatedDatabaseBackupsDataSource() datasource.DataSource {
	return &dedicatedDatabaseBackupsDataSource{}
}

type dedicatedDatabaseBackupsDataSource struct{ client *client.Client }

type dedicatedBackupModel struct {
	ID         types.String `tfsdk:"id"`
	Kind       types.String `tfsdk:"kind"`
	Repo       types.String `tfsdk:"repo"`
	Name       types.String `tfsdk:"name"`
	Status     types.String `tfsdk:"status"`
	SizeBytes  types.Int64  `tfsdk:"size_bytes"`
	StartedAt  types.String `tfsdk:"started_at"`
	FinishedAt types.String `tfsdk:"finished_at"`
}

type dedicatedDatabaseBackupsModel struct {
	DatabaseID      types.String           `tfsdk:"database_id"`
	RetentionDays   types.Int64            `tfsdk:"retention_days"`
	RestorableFrom  types.String           `tfsdk:"restorable_from"`
	RestorableUntil types.String           `tfsdk:"restorable_until"`
	Backups         []dedicatedBackupModel `tfsdk:"backups"`
}

func (d *dedicatedDatabaseBackupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_backups"
}

func (d *dedicatedDatabaseBackupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (d *dedicatedDatabaseBackupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the backups of a Dedicated database and the time window it can be restored to. Use it to pick a target_time or a backup_id for restore_from.",
		Attributes: map[string]schema.Attribute{
			"database_id":      schema.StringAttribute{Required: true},
			"retention_days":   schema.Int64Attribute{Computed: true},
			"restorable_from":  schema.StringAttribute{Computed: true, Description: "The earliest moment you can restore to, as an RFC 3339 time. Empty until a first backup exists."},
			"restorable_until": schema.StringAttribute{Computed: true, Description: "The latest moment you can restore to, as an RFC 3339 time."},
			"backups": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true},
				"kind":        schema.StringAttribute{Computed: true, Description: "scheduled, manual or final."},
				"repo":        schema.StringAttribute{Computed: true},
				"name":        schema.StringAttribute{Computed: true},
				"status":      schema.StringAttribute{Computed: true},
				"size_bytes":  schema.Int64Attribute{Computed: true},
				"started_at":  schema.StringAttribute{Computed: true},
				"finished_at": schema.StringAttribute{Computed: true},
			}}},
		},
	}
}

func (d *dedicatedDatabaseBackupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state dedicatedDatabaseBackupsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	backups, err := d.client.ListDedicatedBackups(ctx, state.DatabaseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift dedicated database backups failed", dedicatedErrorDetail(err))
		return
	}
	state.RetentionDays = types.Int64Value(backups.RetentionDays)
	state.RestorableFrom, state.RestorableUntil = types.StringNull(), types.StringNull()
	if backups.Restorable.Earliest != nil {
		state.RestorableFrom = nonEmptyString(*backups.Restorable.Earliest)
	}
	if backups.Restorable.Latest != nil {
		state.RestorableUntil = nonEmptyString(*backups.Restorable.Latest)
	}
	state.Backups = []dedicatedBackupModel{}
	for _, backup := range backups.Backups {
		item := dedicatedBackupModel{ID: types.StringValue(backup.ID), Kind: types.StringValue(backup.Kind), Repo: types.StringValue(backup.Repo),
			Status: types.StringValue(backup.Status), SizeBytes: types.Int64Value(backup.SizeBytes), Name: stringValue(backup.Name),
			StartedAt: stringValue(backup.StartedAt), FinishedAt: stringValue(backup.FinishedAt)}
		state.Backups = append(state.Backups, item)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
