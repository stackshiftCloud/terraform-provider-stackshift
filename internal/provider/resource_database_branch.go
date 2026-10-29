package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*databaseBranchResource)(nil)
var _ resource.ResourceWithImportState = (*databaseBranchResource)(nil)
var _ resource.ResourceWithValidateConfig = (*databaseBranchResource)(nil)
var _ resource.ResourceWithModifyPlan = (*databaseBranchResource)(nil)

func NewDatabaseBranchResource() resource.Resource { return &databaseBranchResource{} }

type databaseBranchResource struct{ client *client.Client }

type databaseBranchModel struct {
	ID                 types.String   `tfsdk:"id"`
	DatabaseID         types.String   `tfsdk:"database_id"`
	Name               types.String   `tfsdk:"name"`
	From               types.Object   `tfsdk:"from"`
	TTL                types.String   `tfsdk:"ttl"`
	AcceptPriceChanges types.Bool     `tfsdk:"accept_price_changes"`
	InstanceClass      types.String   `tfsdk:"instance_class"`
	SleepAfterMinutes  types.Int64    `tfsdk:"sleep_after_minutes"`
	SourceRef          types.String   `tfsdk:"source_ref"`
	ExpiresAt          types.String   `tfsdk:"expires_at"`
	RootDatabaseID     types.String   `tfsdk:"root_database_id"`
	Mechanism          types.String   `tfsdk:"mechanism"`
	State              types.String   `tfsdk:"state"`
	PrivateHostname    types.String   `tfsdk:"private_hostname"`
	BranchedAt         types.String   `tfsdk:"branched_at"`
	Timeouts           timeouts.Value `tfsdk:"timeouts"`
}

type databaseBranchFromModel struct {
	Now            types.Bool   `tfsdk:"now"`
	Time           types.String `tfsdk:"time"`
	RestorePointID types.String `tfsdk:"restore_point_id"`
	BackupID       types.String `tfsdk:"backup_id"`
}

var branchFromTypes = map[string]attr.Type{
	"now": types.BoolType, "time": types.StringType, "restore_point_id": types.StringType, "backup_id": types.StringType,
}

func (r *databaseBranchResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_branch"
}

func (r *databaseBranchResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *databaseBranchResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "A branch of a PostgreSQL database: a separate working copy that you can change, test against and delete without touching the original. A branch is a database of its own with its own connection details; read them with the stackshift_database_credentials data source and the branch's id. Making a branch the main database isn't available in Terraform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: keep,
				Description: "The branch's own database ID. Use it as database_id in stackshift_database_credentials, or as the parent of another branch."},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString(),
				Description: "The database to branch from. It can be a main database or another branch. Changing this replaces the branch."},
			"name": schema.StringAttribute{Required: true, Description: "The branch's name. Changes in place."},
			"from": schema.SingleNestedAttribute{
				Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown(), objectplanmodifier.RequiresReplace()},
				Description:   "Where the branch's data starts from. Set exactly one of now, time, restore_point_id or backup_id. Defaults to now. Changing this replaces the branch.",
				Attributes: map[string]schema.Attribute{
					"now":              schema.BoolAttribute{Optional: true, Description: "Start from the database as it is right now. Set to true."},
					"time":             schema.StringAttribute{Optional: true, Description: "Start from the database as it was at this moment, an RFC 3339 time inside the database's history window."},
					"restore_point_id": schema.StringAttribute{Optional: true, Description: "Start from this restore point."},
					"backup_id":        schema.StringAttribute{Optional: true, Description: "Start from this backup."},
				},
			},
			"ttl": schema.StringAttribute{Optional: true,
				Description: "How long the branch lives before StackShift deletes it, such as 24h or 7d, or never. Leave it out for the plan's default. Removing it after it was set changes the branch to never. Changes in place."},
			"instance_class": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()},
				Description:   "For a branch of a Dedicated database: the size the branch is built and priced at, a slug from stackshift_database_instance_classes. Leave it out to use the parent's size. Empty for branches of Starter databases. Changing it replaces the branch."},
			"accept_price_changes": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Allow a branch of a Dedicated database, which costs extra, to be charged to the wallet. When false, creating one stops with the quote. Branches of Starter databases are never charged. Only used when the branch is created."},
			"expires_at":          schema.StringAttribute{Computed: true, Description: "When the branch is deleted. Empty when it never expires."},
			"sleep_after_minutes": sleepAfterMinutesAttribute("Sleep when idle for the branch: it sleeps after this many minutes without use, 5 to 1440, and wakes on the next connection. Leave it out or set 0 to keep the branch awake. Changes in place."),
			"source_ref": schema.StringAttribute{Optional: true, PlanModifiers: replaceString(),
				Description: "A label that says what the branch is for, such as pr:142. Changing this replaces the branch."},
			"root_database_id": schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "The main database this branch descends from."},
			"mechanism":        schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "How the branch was made: instant, copy, point_in_time or backup."},
			"state":            schema.StringAttribute{Computed: true, Description: "creating, ready, resetting, promoting, failed, deleting or needs_recovery."},
			"private_hostname": schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "The branch's address on StackShift's private network."},
			"branched_at":      schema.StringAttribute{Computed: true, PlanModifiers: keep, Description: "The moment the branch's data was taken from."},
			"timeouts":         timeouts.Attributes(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

func (r *databaseBranchResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state databaseBranchModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.State, plan.ExpiresAt = state.State, state.ExpiresAt
	if plan.InstanceClass.IsUnknown() {
		plan.InstanceClass = state.InstanceClass
	}
	unchanged := resp.Plan
	resp.Diagnostics.Append(unchanged.Set(ctx, &plan)...)
	if !resp.Diagnostics.HasError() && unchanged.Raw.Equal(req.State.Raw) {
		resp.Plan = unchanged
	}
}

func (r *databaseBranchResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateUnpaddedName(ctx, req.Config, "name", &resp.Diagnostics)
	validateSleepAfterMinutes(ctx, req.Config, &resp.Diagnostics)
	var ttl, sourceRef types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("ttl"), &ttl)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("source_ref"), &sourceRef)...)
	if !ttl.IsNull() && !ttl.IsUnknown() && !validBranchTTL(ttl.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("ttl"), "Unsupported value", `Use hours or days, such as "24h" or "7d", or "never".`)
	}
	if !sourceRef.IsNull() && !sourceRef.IsUnknown() && strings.TrimSpace(sourceRef.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("source_ref"), "Value is empty", "source_ref can't be empty. Leave it out or give it a value.")
	}
	var from types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("from"), &from)...)
	if from.IsNull() || from.IsUnknown() {
		return
	}
	var source databaseBranchFromModel
	resp.Diagnostics.Append(from.As(ctx, &source, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() || source.Now.IsUnknown() || source.Time.IsUnknown() || source.RestorePointID.IsUnknown() || source.BackupID.IsUnknown() {
		return
	}
	chosen := 0
	if source.Now.ValueBool() {
		chosen++
	}
	for _, value := range []types.String{source.Time, source.RestorePointID, source.BackupID} {
		if !value.IsNull() {
			chosen++
		}
	}
	if chosen != 1 || !source.Now.IsNull() && !source.Now.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("from"), "Choose where the branch starts from",
			"Set exactly one of now = true, time, restore_point_id or backup_id in from. Leave from out to start from now.")
		return
	}
	if !source.Time.IsNull() {
		if _, err := time.Parse(time.RFC3339, source.Time.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("from").AtName("time"), "Invalid time", "Use an RFC 3339 time such as 2026-10-25T14:30:00Z.")
		}
	}
}

func validBranchTTL(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "never" {
		return true
	}
	if days, found := strings.CutSuffix(text, "d"); found {
		count, err := strconv.Atoi(days)
		return err == nil && count > 0
	}
	parsed, err := time.ParseDuration(text)
	return err == nil && parsed > 0
}

func (r *databaseBranchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan databaseBranchModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	createTimeout, timeoutDiags := plan.Timeouts.Create(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(timeoutDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()
	parentID := plan.DatabaseID.ValueString()
	request := client.CreateDatabaseBranchRequest{
		Name: plan.Name.ValueString(), From: plan.fromRequest(), TTL: plannedString(plan.TTL),
		SleepAfterMinutes: plannedInt64(plan.SleepAfterMinutes), SourceRef: plannedString(plan.SourceRef),
		InstanceClass: plannedString(plan.InstanceClass),
	}
	var op *client.DatabaseBranchOperation
	if !paidChange(&resp.Diagnostics, "new branch", plan.AcceptPriceChanges.ValueBool(), func(confirm *int64) error {
		request.ConfirmChargeMinor = confirm
		var err error
		op, err = r.client.CreateDatabaseBranch(ctx, parentID, request)
		return err
	}) {
		return
	}
	if op.Branch == nil || op.Branch.ID == "" {
		resp.Diagnostics.AddError("Create StackShift database branch failed", "StackShift accepted the request but didn't return the branch.")
		return
	}
	plan.applyBranch(op.Branch)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	waitErr := r.waitForOperation(ctx, parentID, op.ID)
	if waitErr == nil {
		current, err := r.client.GetDatabaseBranch(ctx, parentID, op.Branch.ID)
		if err != nil {
			resp.Diagnostics.AddError("Read StackShift database branch failed", err.Error())
			return
		}
		plan.applyBranch(current)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	var failed *branchOperationFailed
	if !errors.As(waitErr, &failed) {
		resp.Diagnostics.AddError("StackShift database branch didn't finish", waitErr.Error())
		return
	}
	detail := failed.Error()
	current, err := r.client.GetDatabaseBranch(ctx, parentID, op.Branch.ID)
	switch {
	case notFound(err):
		resp.State.RemoveResource(ctx)
	case err == nil && current.State == "failed":
		if current.FailureMessage != nil && failed.message == "" {
			detail = *current.FailureMessage
		}
		if cleanupErr := r.deleteAndWait(ctx, parentID, current.ID); cleanupErr != nil {
			detail += fmt.Sprintf("\n\nThe failed branch %s is still listed under database %s. Delete it there, or with the branch's delete route, before trying again. (%s)", current.ID, parentID, cleanupErr)
		}
		resp.State.RemoveResource(ctx)
	}
	resp.Diagnostics.AddError("StackShift database branch failed", detail)
}

type branchOperationFailed struct{ message string }

func (e *branchOperationFailed) Error() string {
	if e.message == "" {
		return "The branch operation failed."
	}
	return e.message
}

func (r *databaseBranchResource) waitForOperation(ctx context.Context, parentID, operationID string) error {
	return pollDedicated(ctx, func(ctx context.Context) (bool, error) {
		op, err := r.client.GetDatabaseBranchOperation(ctx, parentID, operationID)
		if err != nil {
			return false, err
		}
		switch op.Status {
		case "succeeded":
			return true, nil
		case "failed":
			return false, &branchOperationFailed{message: op.ErrorMessage}
		}
		return false, nil
	})
}

func (r *databaseBranchResource) deleteAndWait(ctx context.Context, parentID, branchID string) error {
	op, err := r.client.DeleteDatabaseBranch(ctx, parentID, branchID)
	if notFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = r.waitForOperation(ctx, parentID, op.ID)
	if notFound(err) {
		return nil
	}
	return err
}

func (r *databaseBranchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state databaseBranchModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	branch, err := r.client.GetDatabaseBranch(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if notFound(err) || err == nil && branch.State == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database branch failed", err.Error())
		return
	}
	state.applyBranch(branch)
	state.Name = types.StringValue(branch.Name)
	state.SourceRef = stringValue(branch.SourceRef)
	state.InstanceClass = types.StringNull()
	if settings := branchDedicated(branch); settings != nil {
		state.InstanceClass = nonEmptyString(settings.InstanceClass)
	}
	state.SleepAfterMinutes = sleepAfterMinutes(state.SleepAfterMinutes, branchSleep(branch))
	if state.From.IsNull() || !branchFromMatches(state.From, branch.Source) {
		state.From = branchFromValue(branch.Source)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseBranchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state databaseBranchModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parentID, id := state.DatabaseID.ValueString(), state.ID.ValueString()
	changes := map[string]any{}
	if !plan.Name.Equal(state.Name) {
		changes["name"] = plan.Name.ValueString()
	}
	if !plan.TTL.Equal(state.TTL) {
		changes["ttl"] = "never"
		if !plan.TTL.IsNull() {
			changes["ttl"] = plan.TTL.ValueString()
		}
	}
	sleepChanged := !plan.SleepAfterMinutes.Equal(state.SleepAfterMinutes)
	sleepOn := plannedInt64(plan.SleepAfterMinutes) > 0
	if sleepChanged && sleepOn {
		changes["sleep_after_minutes"] = plan.SleepAfterMinutes.ValueInt64()
	}
	if len(changes) > 0 {
		if _, err := r.client.UpdateDatabaseBranch(ctx, parentID, id, changes); err != nil {
			resp.Diagnostics.AddError("Update StackShift database branch failed", err.Error())
			return
		}
	}
	if sleepChanged && !sleepOn {
		if err := r.client.SetDatabaseSleep(ctx, id, 0); err != nil {
			resp.Diagnostics.AddError("Update StackShift database branch sleep failed", err.Error())
			return
		}
	}
	current, err := r.client.GetDatabaseBranch(ctx, parentID, id)
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift database branch failed", err.Error())
		return
	}
	plan.applyBranch(current)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseBranchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state databaseBranchModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	deleteTimeout, timeoutDiags := state.Timeouts.Delete(ctx, dedicatedWaitLimit)
	resp.Diagnostics.Append(timeoutDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	if err := r.deleteAndWait(ctx, state.DatabaseID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift database branch failed", err.Error())
	}
}

func (r *databaseBranchResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Use parent_database_id/branch_id.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("accept_price_changes"), false)...)
}

func (m *databaseBranchModel) applyBranch(branch *client.DatabaseBranch) {
	m.ID = types.StringValue(branch.ID)
	m.RootDatabaseID = nonEmptyString(branch.RootDatabaseID)
	m.Mechanism = nonEmptyString(branch.Mechanism)
	m.State = nonEmptyString(branch.State)
	m.BranchedAt = nonEmptyString(branch.BranchedAt)
	m.ExpiresAt = stringValue(branch.ExpiresAt)
	m.PrivateHostname = types.StringNull()
	if branch.Database != nil {
		m.PrivateHostname = nonEmptyString(branch.Database.PrivateHostname)
	}
	if m.InstanceClass.IsUnknown() {
		m.InstanceClass = types.StringNull()
		if settings := branchDedicated(branch); settings != nil {
			m.InstanceClass = nonEmptyString(settings.InstanceClass)
		}
	}
	if m.From.IsUnknown() {
		m.From = branchFromValue(branch.Source)
	}
}

func branchDedicated(branch *client.DatabaseBranch) *client.DedicatedSettings {
	if branch.Database == nil {
		return nil
	}
	return branch.Database.Dedicated
}

func branchSleep(branch *client.DatabaseBranch) *client.DatabaseSleep {
	if branch.Database == nil {
		return nil
	}
	return branch.Database.Sleep
}

func (m *databaseBranchModel) fromRequest() any {
	if m.From.IsNull() || m.From.IsUnknown() {
		return "now"
	}
	attributes := m.From.Attributes()
	for _, name := range []string{"time", "restore_point_id", "backup_id"} {
		if value, ok := attributes[name].(types.String); ok && plannedString(value) != "" {
			return map[string]string{name: value.ValueString()}
		}
	}
	return "now"
}

func branchFromValue(source client.DatabaseBranchSource) types.Object {
	values := map[string]attr.Value{
		"now": types.BoolNull(), "time": types.StringNull(), "restore_point_id": types.StringNull(), "backup_id": types.StringNull(),
	}
	switch source.Kind {
	case "time":
		values["time"] = types.StringValue(source.Time)
	case "restore_point":
		values["restore_point_id"] = types.StringValue(source.RestorePointID)
	case "backup":
		values["backup_id"] = types.StringValue(source.BackupID)
	default:
		values["now"] = types.BoolValue(true)
	}
	return types.ObjectValueMust(branchFromTypes, values)
}

func branchFromMatches(prior types.Object, source client.DatabaseBranchSource) bool {
	attributes := prior.Attributes()
	text := func(name string) string {
		value, _ := attributes[name].(types.String)
		return plannedString(value)
	}
	switch source.Kind {
	case "time":
		return sameMoment(text("time"), source.Time)
	case "restore_point":
		return text("restore_point_id") == source.RestorePointID
	case "backup":
		return text("backup_id") == source.BackupID
	}
	now, _ := attributes["now"].(types.Bool)
	return now.ValueBool()
}

func sameMoment(a, b string) bool {
	first, firstErr := time.Parse(time.RFC3339, a)
	second, secondErr := time.Parse(time.RFC3339, b)
	if firstErr != nil || secondErr != nil {
		return a == b
	}
	return first.Equal(second)
}
