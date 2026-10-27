package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabaseResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabaseResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dedicatedDatabaseResource)(nil)
var _ resource.ResourceWithModifyPlan = (*dedicatedDatabaseResource)(nil)

func NewDedicatedDatabaseResource() resource.Resource { return &dedicatedDatabaseResource{} }

type dedicatedDatabaseResource struct{ client *client.Client }

type dedicatedMaintenanceWindowModel struct {
	DayOfWeek   types.Int64 `tfsdk:"day_of_week"`
	StartMinute types.Int64 `tfsdk:"start_minute"`
}

type dedicatedRestoreFromModel struct {
	SourceDatabaseID types.String `tfsdk:"source_database_id"`
	TargetTime       types.String `tfsdk:"target_time"`
	BackupID         types.String `tfsdk:"backup_id"`
	FromMirror       types.Bool   `tfsdk:"from_mirror"`
}

type dedicatedEndpointModel struct {
	Protocol types.String `tfsdk:"protocol"`
	Network  types.String `tfsdk:"network"`
	Role     types.String `tfsdk:"role"`
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	TLSMode  types.String `tfsdk:"tls_mode"`
	Primary  types.Bool   `tfsdk:"primary"`
}

var maintenanceWindowType = types.ObjectType{AttrTypes: map[string]attr.Type{"day_of_week": types.Int64Type, "start_minute": types.Int64Type}}

var restoreFromAttributeTypes = map[string]attr.Type{
	"source_database_id": types.StringType, "target_time": types.StringType, "backup_id": types.StringType, "from_mirror": types.BoolType,
}

var dedicatedEndpointType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"protocol": types.StringType, "network": types.StringType, "role": types.StringType, "host": types.StringType,
	"port": types.Int64Type, "tls_mode": types.StringType, "primary": types.BoolType,
}}

type dedicatedDatabaseModel struct {
	ID                  types.String   `tfsdk:"id"`
	ProjectID           types.String   `tfsdk:"project_id"`
	Name                types.String   `tfsdk:"name"`
	Engine              types.String   `tfsdk:"engine"`
	Version             types.String   `tfsdk:"version"`
	InstanceClass       types.String   `tfsdk:"instance_class"`
	StorageGB           types.Int64    `tfsdk:"storage_gb"`
	StorageMaxGB        types.Int64    `tfsdk:"storage_max_gb"`
	AllocatedStorageGB  types.Int64    `tfsdk:"allocated_storage_gb"`
	MasterUsername      types.String   `tfsdk:"master_username"`
	DatabaseName        types.String   `tfsdk:"database_name"`
	BackupRetentionDays types.Int64    `tfsdk:"backup_retention_days"`
	DeletionProtection  types.Bool     `tfsdk:"deletion_protection"`
	MaintenanceWindow   types.Object   `tfsdk:"maintenance_window"`
	Billing             types.String   `tfsdk:"billing"`
	Standby             types.String   `tfsdk:"standby"`
	OnStandbyLoss       types.String   `tfsdk:"on_standby_loss"`
	ApplyChanges        types.String   `tfsdk:"apply_changes"`
	AcceptPriceChanges  types.Bool     `tfsdk:"accept_price_changes"`
	SkipFinalSnapshot   types.Bool     `tfsdk:"skip_final_snapshot"`
	MasterPassword      types.String   `tfsdk:"master_password"`
	Status              types.String   `tfsdk:"status"`
	Host                types.String   `tfsdk:"host"`
	Port                types.Int64    `tfsdk:"port"`
	Endpoints           types.List     `tfsdk:"endpoints"`
	BillingStatus       types.String   `tfsdk:"billing_status"`
	CheckoutURL         types.String   `tfsdk:"checkout_url"`
	RestoreFrom         types.Object   `tfsdk:"restore_from"`
	AllowReplace        types.Bool     `tfsdk:"allow_replace"`
	Timeouts            timeouts.Value `tfsdk:"timeouts"`
}

func (r *dedicatedDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database"
}

func (r *dedicatedDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

type keepStateWhenUnsetString struct{}

func (keepStateWhenUnsetString) Description(context.Context) string {
	return "Keeps the recorded value when the setting is left out of the configuration."
}

func (m keepStateWhenUnsetString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepStateWhenUnsetString) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() && !req.State.Raw.IsNull() {
		resp.PlanValue = req.StateValue
	}
}

type createOnlyObject struct{}

func (createOnlyObject) Description(context.Context) string {
	return "Keeps the recorded value once the resource exists, because it only matters at creation."
}

func (m createOnlyObject) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (createOnlyObject) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if !req.State.Raw.IsNull() && (req.ConfigValue.IsNull() || !req.StateValue.IsNull()) {
		resp.PlanValue = req.StateValue
	}
}

func replaceIfSetString() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull()
	}, "Changing this replaces the database.", "Changing this replaces the database.")}
}

func (r *dedicatedDatabaseResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keepString := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepThenReplace := []planmodifier.String{keepStateWhenUnsetString{}, stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "A Dedicated PostgreSQL or MySQL database. Dedicated must be switched on for your account. Prices: https://stackshift.cloud/pricing.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: keepString},
			"project_id": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keepThenReplace},
			"name":       schema.StringAttribute{Required: true, Description: "3 to 40 lowercase letters, digits and dashes. Changes in place."},
			"engine": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keepThenReplace,
				Description: "postgres (default) or mysql. Changing this replaces the database."},
			"version": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keepThenReplace,
				Description: "PostgreSQL 15 to 18 (default 17) or MySQL 8.4 (default). Changing this replaces the database."},
			"instance_class": schema.StringAttribute{Required: true, Description: "A slug from stackshift_database_instance_classes. Changes in place."},
			"storage_gb":     schema.Int64Attribute{Required: true, Description: "10 to 1024. Grows in place; it can't shrink. Growth from autogrow isn't reported as drift."},
			"storage_max_gb": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description: "Autogrow limit. The server default is twice storage_gb, up to 1024. Changes in place."},
			"allocated_storage_gb": schema.Int64Attribute{Computed: true, Description: "The current storage size, including growth from autogrow."},
			"master_username": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: append([]planmodifier.String{keepStateWhenUnsetString{}}, replaceIfSetString()...),
				Description:   "Defaults to app_owner. Changing this replaces the database."},
			"database_name": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keepThenReplace,
				Description: "Defaults to app. Changing this replaces the database."},
			"backup_retention_days": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description: "7 to 35; 7 when left out. Changes in place."},
			"deletion_protection": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "While true, the database can't be deleted. Changes in place."},
			"maintenance_window": schema.ObjectAttribute{
				Optional: true, Computed: true, AttributeTypes: maintenanceWindowType.AttrTypes,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "{ day_of_week = 0 (Sunday) to 6, start_minute = minutes after midnight UTC, 0 to 1439 }. Picked at random when left out. Changes in place.",
			},
			"billing": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("monthly"),
				Description: "monthly, yearly or hourly. Changes in place; a switch between prepaid periods takes effect at the next renewal."},
			"standby": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("off"),
				Description: "off, sync or async. Changes in place."},
			"on_standby_loss": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("degrade"),
				Description: "degrade or block. Changes in place."},
			"apply_changes": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("window"),
				Description: "When a size change happens: now or window (the maintenance window)."},
			"accept_price_changes": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Allow changes that cost extra to be charged to the wallet. When false, a paid change stops with the quote."},
			"skip_final_snapshot": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Delete without taking a final snapshot."},
			"master_password": schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: keepString,
				Description: "Returned only when the database is created, and kept in Terraform state. Use encrypted remote state."},
			"status": schema.StringAttribute{Computed: true},
			"host":   schema.StringAttribute{Computed: true, PlanModifiers: keepString},
			"port":   schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"endpoints": schema.ListNestedAttribute{
				Computed: true, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"protocol": schema.StringAttribute{Computed: true},
					"network":  schema.StringAttribute{Computed: true},
					"role":     schema.StringAttribute{Computed: true},
					"host":     schema.StringAttribute{Computed: true},
					"port":     schema.Int64Attribute{Computed: true},
					"tls_mode": schema.StringAttribute{Computed: true},
					"primary":  schema.BoolAttribute{Computed: true},
				}},
			},
			"billing_status": schema.StringAttribute{Computed: true},
			"checkout_url": schema.StringAttribute{Computed: true, PlanModifiers: keepString,
				Description: "Where the owner pays while status is pending_payment."},
			"restore_from": schema.SingleNestedAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Object{createOnlyObject{}},
				Description:   "Creates the database as a restore of another Dedicated database instead of an empty one. The source isn't changed. Engine, version, user, database name, backup retention and storage limit are taken from the source. It only matters when the database is created: once the database exists, changing or removing it changes nothing.",
				Attributes: map[string]schema.Attribute{
					"source_database_id": schema.StringAttribute{Required: true, Description: "The Dedicated database to restore from."},
					"target_time": schema.StringAttribute{Optional: true,
						Description: "Restore to this moment, as an RFC 3339 time inside the source's restorable window. Use this or backup_id."},
					"backup_id":   schema.StringAttribute{Optional: true, Description: "Restore from this backup of the source. Use this or target_time."},
					"from_mirror": schema.BoolAttribute{Optional: true, Description: "Read the restore from the second backup copy."},
				},
			},
			"allow_replace": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Changing engine or version replaces the database and deletes its data, so Terraform refuses unless this is true. Never sent to StackShift."},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *dedicatedDatabaseResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state dedicatedDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.AllowReplace.ValueBool() {
		for name, values := range map[string][2]types.String{"engine": {plan.Engine, state.Engine}, "version": {plan.Version, state.Version}} {
			if values[0].IsUnknown() || !values[0].IsNull() && !values[0].Equal(values[1]) {
				resp.Diagnostics.AddAttributeError(path.Root(name), "This would delete the database",
					"Changing engine or version, or setting it from a value Terraform only knows during apply, replaces the database and deletes its data. Set allow_replace = true to go ahead, or move the data with restore_from into a new resource instead.")
			}
		}
	}
	var configured types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("restore_from"), &configured)...)
	if !configured.IsNull() && state.RestoreFrom.IsNull() {
		resp.Diagnostics.AddAttributeWarning(path.Root("restore_from"), "restore_from does nothing here",
			"restore_from only matters when a database is created, so adding it to a database that already exists changes nothing.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Status, plan.BillingStatus, plan.AllocatedStorageGB = state.Status, state.BillingStatus, state.AllocatedStorageGB
	unchanged := resp.Plan
	resp.Diagnostics.Append(unchanged.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && unchanged.Raw.Equal(req.State.Raw) {
		resp.Plan = unchanged
	}
}

func (r *dedicatedDatabaseResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	choices := map[string][]string{
		"billing": {"monthly", "yearly", "hourly"}, "standby": {"off", "sync", "async"},
		"on_standby_loss": {"degrade", "block"}, "apply_changes": {"now", "window"},
	}
	for name, allowed := range choices {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if value.IsNull() || value.IsUnknown() || containsString(allowed, value.ValueString()) {
			continue
		}
		resp.Diagnostics.AddAttributeError(path.Root(name), "Unsupported value", fmt.Sprintf("Use one of %v.", allowed))
	}
	var retention types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("backup_retention_days"), &retention)...)
	if !retention.IsNull() && !retention.IsUnknown() && (retention.ValueInt64() < 7 || retention.ValueInt64() > 35) {
		resp.Diagnostics.AddAttributeError(path.Root("backup_retention_days"), "Unsupported value", "Keep backups for 7 to 35 days.")
	}
	validateUnpaddedName(ctx, req.Config, "name", &resp.Diagnostics)
	var engine, version types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("engine"), &engine)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("version"), &version)...)
	if !engine.IsNull() && !engine.IsUnknown() && engine.ValueString() != "postgres" && engine.ValueString() != "mysql" {
		resp.Diagnostics.AddAttributeError(path.Root("engine"), "Unsupported value", "Use postgres or mysql.")
	}
	if !version.IsNull() && !version.IsUnknown() && !versionPattern.MatchString(version.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("version"), "Unsupported value", "Use a version number such as 17 or 8.4.")
	}
	var window types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("maintenance_window"), &window)...)
	if !window.IsNull() && !window.IsUnknown() {
		var parsed dedicatedMaintenanceWindowModel
		if asDiags := window.As(ctx, &parsed, basetypes.ObjectAsOptions{}); asDiags.HasError() {
			resp.Diagnostics.Append(asDiags...)
		} else {
			validateWindowValue(&resp.Diagnostics, "day_of_week", parsed.DayOfWeek, 0, 6, "day_of_week is 0 (Sunday) to 6.")
			validateWindowValue(&resp.Diagnostics, "start_minute", parsed.StartMinute, 0, 1439, "start_minute is minutes after midnight UTC, 0 to 1439.")
		}
	}
	var restore types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("restore_from"), &restore)...)
	if restore.IsNull() || restore.IsUnknown() {
		return
	}
	var source dedicatedRestoreFromModel
	if asDiags := restore.As(ctx, &source, basetypes.ObjectAsOptions{}); asDiags.HasError() {
		resp.Diagnostics.Append(asDiags...)
		return
	}
	for name, value := range map[string]types.String{"target_time": source.TargetTime, "backup_id": source.BackupID} {
		if !value.IsNull() && !value.IsUnknown() && value.ValueString() == "" {
			resp.Diagnostics.AddAttributeError(path.Root("restore_from").AtName(name), "Value is empty", name+" can't be an empty string. Leave it out or give it a value.")
		}
	}
	if source.TargetTime.IsUnknown() || source.BackupID.IsUnknown() {
		return
	}
	if source.TargetTime.IsNull() == source.BackupID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("restore_from"), "Choose what to restore from", "Set exactly one of target_time or backup_id in restore_from, with a value. If you pass the value in from another resource or data source, check that it isn't empty, for example that the source database already has backups.")
	}
	if !source.TargetTime.IsNull() && source.TargetTime.ValueString() != "" {
		if _, err := time.Parse(time.RFC3339, source.TargetTime.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("restore_from").AtName("target_time"), "Invalid time", "Use an RFC 3339 time such as 2026-10-25T14:30:00Z.")
		}
	}
}

func validateWindowValue(diags *diag.Diagnostics, name string, value types.Int64, low, high int64, message string) {
	switch {
	case value.IsUnknown():
	case value.IsNull():
		diags.AddAttributeError(path.Root("maintenance_window"), "Missing value", name+" is required in maintenance_window.")
	case value.ValueInt64() < low || value.ValueInt64() > high:
		diags.AddAttributeError(path.Root("maintenance_window"), "Unsupported value", message)
	}
}

var versionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (r *dedicatedDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, diags := plan.Timeouts.Create(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	result, err := r.createOrRestore(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	apiErr, _ := client.AsDedicatedError(err)
	pendingPayment := apiErr != nil && apiErr.PaymentRequired() && result.Database != nil && result.Database.ID != ""
	if err != nil && !pendingPayment {
		resp.Diagnostics.AddError("Create StackShift dedicated database failed", dedicatedErrorDetail(err))
		return
	}
	plan.applyDatabase(ctx, result.Database, &resp.Diagnostics)
	plan.MasterPassword = types.StringValue(result.MasterPassword)
	plan.CheckoutURL = types.StringNull()
	plan.BillingStatus = types.StringNull()
	if billing, err := r.client.GetDedicatedBilling(ctx, plan.ID.ValueString()); err == nil {
		plan.BillingStatus = nonEmptyString(billing.Status)
	}
	if pendingPayment {
		checkoutURL := result.CheckoutURL
		if checkoutURL == "" && result.Checkout != nil {
			checkoutURL = result.Checkout.PaymentURL
		}
		plan.CheckoutURL = nonEmptyString(checkoutURL)
		resp.Diagnostics.AddWarning("Payment needed to start the database",
			fmt.Sprintf("The database %s was created and is waiting for payment. The account owner pays here: %s\n\n"+
				"It is kept in Terraform state with status pending_payment and is removed if it isn't paid within 24 hours. "+
				"After paying, run terraform apply again to pick up the running database.", plan.ID.ValueString(), checkoutURL))
	}
	plan.resolveUnknown()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if pendingPayment || resp.Diagnostics.HasError() {
		return
	}
	if err := waitDedicatedRunning(ctx, r.client, plan.ID.ValueString(), createTimeout); err != nil {
		resp.Diagnostics.AddError("StackShift dedicated database didn't start", dedicatedErrorDetail(err))
		return
	}
	if db, err := r.client.GetDedicatedDatabase(ctx, plan.ID.ValueString()); err == nil {
		plan.applyDatabase(ctx, db, &resp.Diagnostics)
	}
	plan.resolveUnknown()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabaseResource) createOrRestore(ctx context.Context, plan *dedicatedDatabaseModel, diags *diag.Diagnostics) (*client.DedicatedCreateResult, error) {
	if plan.RestoreFrom.IsNull() || plan.RestoreFrom.IsUnknown() {
		engine := plannedString(plan.Engine)
		if engine == "" {
			engine = "postgres"
		}
		request := client.CreateDedicatedDatabaseRequest{
			Type: engine, Name: plan.Name.ValueString(), Version: plannedString(plan.Version), InstanceClass: plan.InstanceClass.ValueString(),
			StorageGB: plan.StorageGB.ValueInt64(), StorageMaxGB: plannedInt64(plan.StorageMaxGB),
			MasterUsername: plannedString(plan.MasterUsername), DatabaseName: plannedString(plan.DatabaseName),
			BackupRetentionDays: plannedInt64(plan.BackupRetentionDays), DeletionProtection: plan.DeletionProtection.ValueBool(),
			Billing: plan.Billing.ValueString(), Standby: plan.Standby.ValueString(), OnStandbyLoss: plan.OnStandbyLoss.ValueString(),
		}
		request.MaintenanceWindow = plan.maintenanceWindow(ctx, diags)
		if diags.HasError() {
			return nil, nil
		}
		return r.client.CreateDedicatedDatabase(ctx, plannedString(plan.ProjectID), request)
	}
	var source dedicatedRestoreFromModel
	diags.Append(plan.RestoreFrom.As(ctx, &source, basetypes.ObjectAsOptions{})...)
	if conflicts := plan.restoreConflicts(); len(conflicts) > 0 {
		diags.AddError("These settings come from the source database",
			fmt.Sprintf("%s can't be set while restoring. Remove them, create the restore, then set them in a later apply.", strings.Join(conflicts, ", ")))
	}
	if diags.HasError() {
		return nil, nil
	}
	return r.client.RestoreDedicatedDatabase(ctx, source.SourceDatabaseID.ValueString(), client.RestoreDedicatedDatabaseRequest{
		Name: plan.Name.ValueString(), TargetTime: plannedString(source.TargetTime), BackupID: plannedString(source.BackupID),
		InstanceClass: plan.InstanceClass.ValueString(), StorageGB: plan.StorageGB.ValueInt64(), FromMirror: source.FromMirror.ValueBool(),
		Billing: plan.Billing.ValueString(),
	})
}

func (m *dedicatedDatabaseModel) restoreConflicts() []string {
	var conflicts []string
	for name, value := range map[string]attr.Value{
		"engine": m.Engine, "version": m.Version, "master_username": m.MasterUsername, "database_name": m.DatabaseName,
		"backup_retention_days": m.BackupRetentionDays, "storage_max_gb": m.StorageMaxGB, "maintenance_window": m.MaintenanceWindow,
	} {
		if !value.IsNull() && !value.IsUnknown() {
			conflicts = append(conflicts, name)
		}
	}
	if m.DeletionProtection.ValueBool() {
		conflicts = append(conflicts, "deletion_protection")
	}
	if m.Standby.ValueString() != "off" {
		conflicts = append(conflicts, "standby")
	}
	if m.OnStandbyLoss.ValueString() != "degrade" {
		conflicts = append(conflicts, "on_standby_loss")
	}
	sort.Strings(conflicts)
	return conflicts
}

func plannedString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func plannedInt64(value types.Int64) int64 {
	if value.IsNull() || value.IsUnknown() {
		return 0
	}
	return value.ValueInt64()
}

func (r *dedicatedDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	db, err := r.client.GetDedicatedDatabase(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift dedicated database failed", dedicatedErrorDetail(err))
		return
	}
	if db.Status == "deleting" || db.Status == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}
	state.applyDatabase(ctx, db, &resp.Diagnostics)
	ha, err := r.client.GetDedicatedHA(ctx, state.ID.ValueString())
	if err != nil && !notFound(err) {
		resp.Diagnostics.AddError("Read StackShift dedicated database failed", dedicatedErrorDetail(err))
		return
	}
	if err == nil {
		state.applyHA(ha, &resp.Diagnostics)
	}
	billing, err := r.client.GetDedicatedBilling(ctx, state.ID.ValueString())
	if err == nil {
		state.BillingStatus = nonEmptyString(billing.Status)
		if billing.NextBilling != nil && billing.NextBilling.Mode != "" {
			state.Billing = types.StringValue(billing.NextBilling.Mode)
		}
	} else if apiErr, ok := client.AsDedicatedError(err); !notFound(err) && !(ok && apiErr.StatusCode == http.StatusServiceUnavailable) {
		resp.Diagnostics.AddError("Read StackShift dedicated database billing failed", dedicatedErrorDetail(err))
		return
	}
	if state.Status.ValueString() == "pending_payment" {
		resp.Diagnostics.AddWarning("Database is waiting for payment",
			fmt.Sprintf("The database %s won't start until it's paid: %s", state.ID.ValueString(), state.CheckoutURL.ValueString()))
	} else {
		state.CheckoutURL = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dedicatedDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateTimeout, diags := plan.Timeouts.Update(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	working := plan
	working.InstanceClass, working.StorageGB, working.StorageMaxGB = state.InstanceClass, state.StorageGB, state.StorageMaxGB
	working.Standby, working.OnStandbyLoss = state.Standby, state.OnStandbyLoss
	working.BackupRetentionDays, working.DeletionProtection, working.MaintenanceWindow = state.BackupRetentionDays, state.DeletionProtection, state.MaintenanceWindow
	working.Name, working.Billing = state.Name, state.Billing
	working.Status, working.BillingStatus, working.AllocatedStorageGB = state.Status, state.BillingStatus, state.AllocatedStorageGB
	working.resolveUnknown()
	id := plan.ID.ValueString()
	accept := plan.AcceptPriceChanges.ValueBool()
	save := func() { resp.Diagnostics.Append(resp.State.Set(ctx, &working)...) }

	settings := map[string]any{}
	if !plan.Name.Equal(state.Name) {
		settings["name"] = plan.Name.ValueString()
	}
	if !plan.BackupRetentionDays.Equal(state.BackupRetentionDays) {
		settings["backup_retention_days"] = plan.BackupRetentionDays.ValueInt64()
	}
	if !plan.DeletionProtection.Equal(state.DeletionProtection) {
		settings["deletion_protection"] = plan.DeletionProtection.ValueBool()
	}
	if !plan.MaintenanceWindow.IsUnknown() && !plan.MaintenanceWindow.Equal(state.MaintenanceWindow) {
		settings["maintenance_window"] = plan.maintenanceWindow(ctx, &resp.Diagnostics)
	}
	if len(settings) > 0 {
		if _, err := r.client.UpdateDedicatedSettings(ctx, id, settings); err != nil {
			resp.Diagnostics.AddError("Update StackShift dedicated database settings failed", dedicatedErrorDetail(err))
		}
		if resp.Diagnostics.HasError() {
			save()
			return
		}
		working.Name = plan.Name
		working.BackupRetentionDays, working.DeletionProtection, working.MaintenanceWindow = plan.BackupRetentionDays, plan.DeletionProtection, plan.MaintenanceWindow
	}

	grow := plan.StorageGB.ValueInt64() > state.StorageGB.ValueInt64()
	if grow || (!plan.StorageMaxGB.IsNull() && !plan.StorageMaxGB.Equal(state.StorageMaxGB)) {
		var storageGB, storageMaxGB int64
		if grow {
			storageGB = plan.StorageGB.ValueInt64()
		}
		if !plan.StorageMaxGB.IsUnknown() && !plan.StorageMaxGB.Equal(state.StorageMaxGB) {
			storageMaxGB = plan.StorageMaxGB.ValueInt64()
		}
		if !paidChange(&resp.Diagnostics, "storage change", accept, func(confirm *int64) error {
			return r.client.UpdateDedicatedStorage(ctx, id, storageGB, storageMaxGB, confirm)
		}) {
			save()
			return
		}
	}
	working.StorageGB, working.StorageMaxGB = plan.StorageGB, plan.StorageMaxGB

	if !plan.Standby.Equal(state.Standby) || !plan.OnStandbyLoss.Equal(state.OnStandbyLoss) {
		if !paidChange(&resp.Diagnostics, "standby change", accept, func(confirm *int64) error {
			return r.client.SetDedicatedStandby(ctx, id, plan.Standby.ValueString(), plan.OnStandbyLoss.ValueString(), confirm)
		}) {
			save()
			return
		}
		working.Standby, working.OnStandbyLoss = plan.Standby, plan.OnStandbyLoss
	}

	if !plan.InstanceClass.Equal(state.InstanceClass) {
		if !paidChange(&resp.Diagnostics, "size change", accept, func(confirm *int64) error {
			return r.client.ChangeDedicatedClass(ctx, id, plan.InstanceClass.ValueString(), plan.ApplyChanges.ValueString(), confirm)
		}) {
			save()
			return
		}
		working.InstanceClass = plan.InstanceClass
	}

	if !plan.Billing.Equal(state.Billing) {
		if !paidChange(&resp.Diagnostics, "billing change", accept, func(confirm *int64) error {
			return r.client.SetDedicatedBilling(ctx, id, plan.Billing.ValueString(), confirm)
		}) {
			save()
			return
		}
		working.Billing = plan.Billing
	}

	if db, err := r.client.GetDedicatedDatabase(ctx, id); err == nil && dedicatedTransitional(db.Status) {
		if err := waitDedicatedRunning(ctx, r.client, id, updateTimeout); err != nil {
			resp.Diagnostics.AddError("StackShift dedicated database didn't finish the change", dedicatedErrorDetail(err))
			save()
			return
		}
	}
	if db, err := r.client.GetDedicatedDatabase(ctx, id); err == nil {
		working.Status = types.StringValue(db.Status)
		if db.Dedicated != nil {
			working.AllocatedStorageGB = types.Int64Value(db.Dedicated.StorageGB)
		}
	}
	if billing, err := r.client.GetDedicatedBilling(ctx, id); err == nil {
		working.BillingStatus = nonEmptyString(billing.Status)
	}
	save()
}

func (r *dedicatedDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, diags := state.Timeouts.Delete(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if err := r.client.DeleteDedicatedDatabase(ctx, id, state.SkipFinalSnapshot.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift dedicated database failed", dedicatedErrorDetail(err))
		return
	}
	err := pollDedicated(ctx, deleteTimeout, func(ctx context.Context) (bool, error) {
		db, err := r.client.GetDedicatedDatabase(ctx, id)
		if notFound(err) {
			return true, nil
		}
		return err == nil && db.Status == "deleted", err
	})
	if err != nil {
		resp.Diagnostics.AddError("StackShift dedicated database wasn't deleted", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("apply_changes"), "window")...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("accept_price_changes"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("skip_final_snapshot"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("allow_replace"), false)...)
}

func (m *dedicatedDatabaseModel) applyDatabase(ctx context.Context, db *client.DedicatedDatabase, diags *diag.Diagnostics) {
	m.ID = types.StringValue(db.ID)
	if db.ProjectID != nil && *db.ProjectID != "" {
		m.ProjectID = types.StringValue(*db.ProjectID)
	}
	m.Name = firstKnownString(db.Name, m.Name)
	m.Engine = firstKnownString(db.Type, m.Engine)
	m.Version = firstKnownString(db.Version, m.Version)
	m.DatabaseName = firstKnownString(db.DatabaseName, m.DatabaseName)
	m.MasterUsername = firstKnownString(db.Username, m.MasterUsername)
	m.Status = types.StringValue(db.Status)
	m.Host = nonEmptyString(db.Host)
	m.Port = types.Int64Null()
	if db.Port > 0 {
		m.Port = types.Int64Value(db.Port)
	}
	endpoints := []dedicatedEndpointModel{}
	for _, endpoint := range db.Endpoints {
		endpoints = append(endpoints, dedicatedEndpointModel{
			Protocol: types.StringValue(endpoint.Protocol), Network: types.StringValue(endpoint.Network), Role: types.StringValue(endpoint.Role),
			Host: types.StringValue(endpoint.Host), Port: types.Int64Value(endpoint.Port), TLSMode: types.StringValue(endpoint.TLSMode),
			Primary: types.BoolValue(endpoint.Primary),
		})
	}
	list, d := types.ListValueFrom(ctx, dedicatedEndpointType, endpoints)
	diags.Append(d...)
	m.Endpoints = list
	if settings := db.Dedicated; settings != nil {
		m.InstanceClass = firstKnownString(settings.InstanceClass, m.InstanceClass)
		m.AllocatedStorageGB = types.Int64Value(settings.StorageGB)
		if m.StorageGB.IsNull() || m.StorageGB.IsUnknown() || settings.StorageGB < m.StorageGB.ValueInt64() {
			m.StorageGB = types.Int64Value(settings.StorageGB)
		}
		m.StorageMaxGB = types.Int64Value(settings.StorageMaxGB)
		m.BackupRetentionDays = types.Int64Value(settings.BackupRetentionDays)
		m.DeletionProtection = types.BoolValue(settings.DeletionProtection)
		window, d := types.ObjectValue(maintenanceWindowType.AttrTypes, map[string]attr.Value{
			"day_of_week": types.Int64Value(settings.MaintenanceWindow.DayOfWeek), "start_minute": types.Int64Value(settings.MaintenanceWindow.StartMinute),
		})
		diags.Append(d...)
		m.MaintenanceWindow = window
		m.Billing = firstKnownString(settings.Billing, m.Billing)
		m.Standby = firstKnownString(settings.Standby, m.Standby)
		m.OnStandbyLoss = firstKnownString(settings.OnStandbyLoss, m.OnStandbyLoss)
	}
}

func (m *dedicatedDatabaseModel) maintenanceWindow(ctx context.Context, diags *diag.Diagnostics) *client.DedicatedMaintenanceWindow {
	if m.MaintenanceWindow.IsNull() || m.MaintenanceWindow.IsUnknown() {
		return nil
	}
	var window dedicatedMaintenanceWindowModel
	diags.Append(m.MaintenanceWindow.As(ctx, &window, basetypes.ObjectAsOptions{})...)
	return &client.DedicatedMaintenanceWindow{DayOfWeek: window.DayOfWeek.ValueInt64(), StartMinute: window.StartMinute.ValueInt64()}
}

func (m *dedicatedDatabaseModel) applyHA(ha *client.DedicatedHA, diags *diag.Diagnostics) {
	for _, member := range ha.Members {
		if member.Role == "primary" && member.InstanceClass != "" {
			m.InstanceClass = types.StringValue(member.InstanceClass)
		}
	}
	change := ha.SizeChange
	if change == nil {
		return
	}
	switch change.Status {
	case "scheduled", "adding", "switching", "removing":
		m.InstanceClass = firstKnownString(change.ToClass, m.InstanceClass)
	case "failed":
		diags.AddWarning("Size change failed", fmt.Sprintf("Changing to %s failed: %s", change.ToClass, change.FailureMessage))
	}
}

func (m *dedicatedDatabaseModel) resolveUnknown() {
	for _, value := range []*types.String{&m.MasterPassword, &m.Host, &m.CheckoutURL, &m.Status, &m.BillingStatus,
		&m.ProjectID, &m.Engine, &m.Version, &m.MasterUsername, &m.DatabaseName} {
		if value.IsUnknown() {
			*value = types.StringNull()
		}
	}
	if m.AllocatedStorageGB.IsUnknown() {
		m.AllocatedStorageGB = types.Int64Null()
	}
	if m.StorageMaxGB.IsUnknown() {
		m.StorageMaxGB = types.Int64Null()
	}
	if m.RestoreFrom.IsUnknown() {
		m.RestoreFrom = types.ObjectNull(restoreFromAttributeTypes)
	}
	if m.BackupRetentionDays.IsUnknown() {
		m.BackupRetentionDays = types.Int64Null()
	}
	if m.MaintenanceWindow.IsUnknown() {
		m.MaintenanceWindow = types.ObjectNull(maintenanceWindowType.AttrTypes)
	}
	if m.Port.IsUnknown() {
		m.Port = types.Int64Null()
	}
	if m.Endpoints.IsUnknown() {
		m.Endpoints = types.ListNull(dedicatedEndpointType)
	}
}

func firstKnownString(value string, fallback types.String) types.String {
	if value != "" {
		return types.StringValue(value)
	}
	return fallback
}

func nonEmptyString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func formatMinor(amount int64, currency string) string {
	sign := ""
	if amount < 0 {
		sign, amount = "-", -amount
	}
	return fmt.Sprintf("%s%s %d.%02d", sign, currency, amount/100, amount%100)
}

func dedicatedErrorDetail(err error) string {
	apiErr, ok := client.AsDedicatedError(err)
	if !ok {
		return err.Error()
	}
	switch {
	case apiErr.StatusCode == http.StatusForbidden:
		return apiErr.Message + "\n\nDedicated databases must be switched on for your account. Contact StackShift support to turn them on."
	case apiErr.PaymentRequired():
		detail := "Your wallet doesn't cover this change. Top up your wallet, then run terraform apply again."
		if apiErr.AmountMinor > 0 {
			detail = fmt.Sprintf("Your wallet doesn't cover this change (%s). Top up your wallet, then run terraform apply again.",
				formatMinor(apiErr.AmountMinor, apiErr.Currency))
		}
		if apiErr.CheckoutURL != "" {
			detail += "\n\nPay here: " + apiErr.CheckoutURL
		}
		return detail
	}
	return apiErr.Error()
}

func paidChange(diags *diag.Diagnostics, label string, accept bool, send func(confirm *int64) error) bool {
	err := send(nil)
	if apiErr, ok := client.AsDedicatedError(err); ok && apiErr.PriceConfirmationRequired() {
		quote := apiErr.Quote
		summary := fmt.Sprintf("Charge %s now; the new price is %s a month (%s billing).",
			formatMinor(quote.ChargeMinor, quote.Currency), formatMinor(quote.NewMonthlyMinor, quote.Currency), quote.Billing)
		if !accept {
			diags.AddError("The "+label+" costs extra", summary+"\n\nSet accept_price_changes = true and apply again to go ahead. Prices: https://stackshift.cloud/pricing")
			return false
		}
		charge := quote.ChargeMinor
		if err = send(&charge); err == nil {
			diags.AddWarning("The "+label+" was charged to your wallet", summary)
			return true
		}
	}
	if err != nil {
		diags.AddError("The "+label+" failed", dedicatedErrorDetail(err))
		return false
	}
	return true
}
