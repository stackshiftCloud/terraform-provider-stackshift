package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
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

func NewDedicatedDatabaseResource() resource.Resource { return &dedicatedDatabaseResource{} }

type dedicatedDatabaseResource struct{ client *client.Client }

type dedicatedMaintenanceWindowModel struct {
	DayOfWeek   types.Int64 `tfsdk:"day_of_week"`
	StartMinute types.Int64 `tfsdk:"start_minute"`
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

var dedicatedEndpointType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"protocol": types.StringType, "network": types.StringType, "role": types.StringType, "host": types.StringType,
	"port": types.Int64Type, "tls_mode": types.StringType, "primary": types.BoolType,
}}

type dedicatedDatabaseModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Name                types.String `tfsdk:"name"`
	Version             types.String `tfsdk:"version"`
	InstanceClass       types.String `tfsdk:"instance_class"`
	StorageGB           types.Int64  `tfsdk:"storage_gb"`
	StorageMaxGB        types.Int64  `tfsdk:"storage_max_gb"`
	AllocatedStorageGB  types.Int64  `tfsdk:"allocated_storage_gb"`
	MasterUsername      types.String `tfsdk:"master_username"`
	DatabaseName        types.String `tfsdk:"database_name"`
	BackupRetentionDays types.Int64  `tfsdk:"backup_retention_days"`
	DeletionProtection  types.Bool   `tfsdk:"deletion_protection"`
	MaintenanceWindow   types.Object `tfsdk:"maintenance_window"`
	Billing             types.String `tfsdk:"billing"`
	Standby             types.String `tfsdk:"standby"`
	OnStandbyLoss       types.String `tfsdk:"on_standby_loss"`
	ApplyChanges        types.String `tfsdk:"apply_changes"`
	AcceptPriceChanges  types.Bool   `tfsdk:"accept_price_changes"`
	SkipFinalSnapshot   types.Bool   `tfsdk:"skip_final_snapshot"`
	MasterPassword      types.String `tfsdk:"master_password"`
	Status              types.String `tfsdk:"status"`
	Host                types.String `tfsdk:"host"`
	Port                types.Int64  `tfsdk:"port"`
	Endpoints           types.List   `tfsdk:"endpoints"`
	BillingStatus       types.String `tfsdk:"billing_status"`
	CheckoutURL         types.String `tfsdk:"checkout_url"`
}

func (r *dedicatedDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database"
}

func (r *dedicatedDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func replaceIfSetString() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = !req.StateValue.IsNull()
	}, "Changing this replaces the database.", "Changing this replaces the database.")}
}

func (r *dedicatedDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keepString := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A Dedicated PostgreSQL database. Dedicated must be switched on for your account. Prices: https://stackshift.cloud/pricing.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true, PlanModifiers: keepString},
			"project_id":     schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
			"name":           schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"version":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("17"), PlanModifiers: replaceString()},
			"instance_class": schema.StringAttribute{Required: true, Description: "A slug from stackshift_database_instance_classes. Changes in place."},
			"storage_gb":     schema.Int64Attribute{Required: true, Description: "10 to 1024. Grows in place; it can't shrink. Growth from autogrow isn't reported as drift."},
			"storage_max_gb": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description: "Autogrow limit. The server default is twice storage_gb, up to 1024. Changes in place."},
			"allocated_storage_gb": schema.Int64Attribute{Computed: true, Description: "The current storage size, including growth from autogrow."},
			"master_username": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("app_owner"),
				PlanModifiers: replaceIfSetString()},
			"database_name": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("app"), PlanModifiers: replaceString()},
			"backup_retention_days": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(7),
				Description: "7 to 35. Changes in place."},
			"deletion_protection": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "While true, the database can't be deleted. Changes in place."},
			"maintenance_window": schema.SingleNestedAttribute{
				Optional: true, Computed: true, PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description: "Picked at random when left out. Changes in place.",
				Attributes: map[string]schema.Attribute{
					"day_of_week":  schema.Int64Attribute{Required: true, Description: "0 (Sunday) to 6."},
					"start_minute": schema.Int64Attribute{Required: true, Description: "Minutes after midnight UTC, 0 to 1439."},
				},
			},
			"billing": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("monthly"),
				PlanModifiers: replaceString(), Description: "monthly, yearly or hourly."},
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
		},
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
}

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
	request := client.CreateDedicatedDatabaseRequest{
		Name: plan.Name.ValueString(), Version: plan.Version.ValueString(), InstanceClass: plan.InstanceClass.ValueString(),
		StorageGB: plan.StorageGB.ValueInt64(), StorageMaxGB: plan.StorageMaxGB.ValueInt64(),
		MasterUsername: plan.MasterUsername.ValueString(), DatabaseName: plan.DatabaseName.ValueString(),
		BackupRetentionDays: plan.BackupRetentionDays.ValueInt64(), DeletionProtection: plan.DeletionProtection.ValueBool(),
		Billing: plan.Billing.ValueString(), Standby: plan.Standby.ValueString(), OnStandbyLoss: plan.OnStandbyLoss.ValueString(),
	}
	request.MaintenanceWindow = plan.maintenanceWindow(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.CreateDedicatedDatabase(ctx, plan.ProjectID.ValueString(), request)
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
	working := plan
	working.InstanceClass, working.StorageGB, working.StorageMaxGB = state.InstanceClass, state.StorageGB, state.StorageMaxGB
	working.Standby, working.OnStandbyLoss = state.Standby, state.OnStandbyLoss
	working.BackupRetentionDays, working.DeletionProtection, working.MaintenanceWindow = state.BackupRetentionDays, state.DeletionProtection, state.MaintenanceWindow
	working.Status, working.BillingStatus, working.AllocatedStorageGB = state.Status, state.BillingStatus, state.AllocatedStorageGB
	working.resolveUnknown()
	id := plan.ID.ValueString()
	accept := plan.AcceptPriceChanges.ValueBool()
	save := func() { resp.Diagnostics.Append(resp.State.Set(ctx, &working)...) }

	settings := map[string]any{}
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
	if err := r.client.DeleteDedicatedDatabase(ctx, state.ID.ValueString(), state.SkipFinalSnapshot.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift dedicated database failed", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *dedicatedDatabaseModel) applyDatabase(ctx context.Context, db *client.DedicatedDatabase, diags *diag.Diagnostics) {
	m.ID = types.StringValue(db.ID)
	if db.ProjectID != nil && *db.ProjectID != "" {
		m.ProjectID = types.StringValue(*db.ProjectID)
	}
	m.Name = firstKnownString(db.Name, m.Name)
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
	for _, value := range []*types.String{&m.MasterPassword, &m.Host, &m.CheckoutURL, &m.Status, &m.BillingStatus} {
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
