package provider

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.ResourceWithConfigure = (*dedicatedDatabasePublicAccessResource)(nil)
var _ resource.ResourceWithImportState = (*dedicatedDatabasePublicAccessResource)(nil)
var _ resource.ResourceWithValidateConfig = (*dedicatedDatabasePublicAccessResource)(nil)

var utcTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

func NewDedicatedDatabasePublicAccessResource() resource.Resource {
	return &dedicatedDatabasePublicAccessResource{}
}

type dedicatedDatabasePublicAccessResource struct{ client *client.Client }

type dedicatedDatabasePublicAccessModel struct {
	ID                        types.String   `tfsdk:"id"`
	DatabaseID                types.String   `tfsdk:"database_id"`
	AccessMode                types.String   `tfsdk:"access_mode"`
	AllowedCIDRs              types.Set      `tfsdk:"allowed_cidrs"`
	ExpiresAt                 types.String   `tfsdk:"expires_at"`
	AcknowledgePublicExposure types.Bool     `tfsdk:"acknowledge_public_exposure"`
	Status                    types.String   `tfsdk:"status"`
	PublicHostname            types.String   `tfsdk:"public_hostname"`
	PublicPort                types.Int64    `tfsdk:"public_port"`
	Timeouts                  timeouts.Value `tfsdk:"timeouts"`
}

func (r *dedicatedDatabasePublicAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedicated_database_public_access"
}

func (r *dedicatedDatabasePublicAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *dedicatedDatabasePublicAccessResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepThenReplace := []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Lets people outside StackShift connect to a Dedicated database through a secure public address. The database must be running. Destroying this resource turns public access off. Changing any setting replaces the access, which briefly turns it off.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"database_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"access_mode": schema.StringAttribute{Required: true, PlanModifiers: replaceString(),
				Description: "restricted lets only allowed_cidrs connect until expires_at. anywhere lets any address connect and renews itself, and needs acknowledge_public_exposure."},
			"allowed_cidrs": schema.SetAttribute{Optional: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
				Description: "Address ranges that may connect, such as 203.0.113.0/24. Required for restricted."},
			"expires_at": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: keepThenReplace,
				Description: "When restricted access ends, as a UTC time such as 2026-12-01T00:00:00Z, between 1 hour from now and the platform limit. Required for restricted. For anywhere it's the current renewal time."},
			"acknowledge_public_exposure": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description: "Set to true to confirm that anywhere opens the database to the internet. Only used with anywhere."},
			"status":          schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"public_hostname": schema.StringAttribute{Computed: true, PlanModifiers: keep},
			"public_port":     schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"timeouts":        timeouts.Attributes(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

func (r *dedicatedDatabasePublicAccessResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var mode, expires types.String
	var cidrs types.Set
	var acknowledged types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("access_mode"), &mode)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("allowed_cidrs"), &cidrs)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("expires_at"), &expires)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("acknowledge_public_exposure"), &acknowledged)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !expires.IsNull() && !expires.IsUnknown() && !utcTimestamp.MatchString(expires.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid time", "Use a UTC time such as 2026-12-01T00:00:00Z.")
	}
	if !cidrs.IsNull() && !cidrs.IsUnknown() {
		var entries []types.String
		resp.Diagnostics.Append(cidrs.ElementsAs(ctx, &entries, false)...)
		for _, entry := range entries {
			if !entry.IsUnknown() && !canonicalCIDR(entry.ValueString()) {
				resp.Diagnostics.AddAttributeError(path.Root("allowed_cidrs"), "Use canonical address ranges",
					fmt.Sprintf("%q isn't a network written the way StackShift stores it, such as 203.0.113.0/24. Use the network address with its prefix, and not 0.0.0.0/0.", entry.ValueString()))
			}
		}
	}
	if mode.IsNull() || mode.IsUnknown() {
		return
	}
	switch mode.ValueString() {
	case "restricted":
		if cidrs.IsNull() || expires.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("access_mode"), "Restricted access needs a limit", "Set allowed_cidrs and expires_at for restricted access.")
		}
		if acknowledged.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("acknowledge_public_exposure"), "Not used here", "acknowledge_public_exposure is only for anywhere access.")
		}
	case "anywhere":
		if (!cidrs.IsNull() && !cidrs.IsUnknown()) || (!expires.IsNull() && !expires.IsUnknown()) {
			resp.Diagnostics.AddAttributeError(path.Root("access_mode"), "Anywhere access takes no limit", "Leave out allowed_cidrs and expires_at for anywhere access.")
		}
		if !acknowledged.IsUnknown() && !acknowledged.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("acknowledge_public_exposure"), "Confirm public access", "Set acknowledge_public_exposure = true to open the database to any address.")
		}
	default:
		resp.Diagnostics.AddAttributeError(path.Root("access_mode"), "Unsupported value", "Use restricted or anywhere.")
	}
}

func canonicalCIDR(value string) bool {
	ip, network, err := net.ParseCIDR(value)
	if err != nil {
		return false
	}
	ones, _ := network.Mask.Size()
	return ones > 0 && ip.Equal(network.IP) && network.String() == value
}

func (r *dedicatedDatabasePublicAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dedicatedDatabasePublicAccessModel
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
	databaseID := plan.DatabaseID.ValueString()
	if err := waitDedicatedRunning(ctx, r.client, databaseID); err != nil {
		resp.Diagnostics.AddError("The database isn't ready for public access", dedicatedErrorDetail(err))
		return
	}
	request := client.DedicatedPublicAccessRequest{
		Transport: "tunnel", AccessMode: plan.AccessMode.ValueString(), ExpiresAt: plannedString(plan.ExpiresAt),
		AcknowledgePublicExposure: plan.AcknowledgePublicExposure.ValueBool(),
	}
	if !plan.AllowedCIDRs.IsNull() {
		resp.Diagnostics.Append(plan.AllowedCIDRs.ElementsAs(ctx, &request.AllowedCIDRs, false)...)
		sort.Strings(request.AllowedCIDRs)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.CreateDedicatedPublicAccess(ctx, databaseID, request)
	if err != nil {
		resp.Diagnostics.AddError("Turn on StackShift public access failed", dedicatedErrorDetail(err))
		return
	}
	plan.apply(ctx, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	err = pollDedicated(ctx, func(ctx context.Context) (bool, error) {
		current, err := r.client.GetDedicatedPublicAccess(ctx, databaseID, policy.ID)
		if err != nil {
			return false, err
		}
		plan.apply(ctx, current, &resp.Diagnostics)
		switch current.Status {
		case "active":
			return true, nil
		case "pending", "applying":
			return false, nil
		}
		message := "public access is " + current.Status
		if current.ErrorMessage != nil {
			message = *current.ErrorMessage
		}
		return false, fmt.Errorf("%s", message)
	})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("StackShift public access didn't turn on", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabasePublicAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dedicatedDatabasePublicAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policy, err := r.client.GetDedicatedPublicAccess(ctx, state.DatabaseID.ValueString(), state.ID.ValueString())
	if notFound(err) || err == nil && (policy.Status == "revoked" || policy.Status == "expired") {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift public access failed", dedicatedErrorDetail(err))
		return
	}
	state.apply(ctx, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dedicatedDatabasePublicAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dedicatedDatabasePublicAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dedicatedDatabasePublicAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dedicatedDatabasePublicAccessModel
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
	databaseID, id := state.DatabaseID.ValueString(), state.ID.ValueString()
	if err := r.client.RevokeDedicatedPublicAccess(ctx, databaseID, id); err != nil {
		resp.Diagnostics.AddError("Turn off StackShift public access failed", dedicatedErrorDetail(err))
		return
	}
	err := pollDedicated(ctx, func(ctx context.Context) (bool, error) {
		policy, err := r.client.GetDedicatedPublicAccess(ctx, databaseID, id)
		if notFound(err) || err == nil && (policy.Status == "revoked" || policy.Status == "expired") {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		resp.Diagnostics.AddError("StackShift public access didn't turn off", dedicatedErrorDetail(err))
	}
}

func (r *dedicatedDatabasePublicAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "Use database_id:policy_id. "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (m *dedicatedDatabasePublicAccessModel) apply(ctx context.Context, policy *client.DedicatedPublicAccessPolicy, diags *diag.Diagnostics) {
	m.ID = types.StringValue(policy.ID)
	m.AccessMode = types.StringValue(policy.AccessMode)
	m.Status = types.StringValue(policy.Status)
	m.PublicHostname = nonEmptyString(policy.PublicHostname)
	m.PublicPort = types.Int64Null()
	if policy.PublicPort > 0 {
		m.PublicPort = types.Int64Value(policy.PublicPort)
	}
	if !sameInstant(m.ExpiresAt, policy.ExpiresAt) {
		m.ExpiresAt = nonEmptyString(policy.ExpiresAt)
	}
	m.AllowedCIDRs = types.SetNull(types.StringType)
	if len(policy.AllowedCIDRs) > 0 {
		set, d := types.SetValueFrom(ctx, types.StringType, policy.AllowedCIDRs)
		diags.Append(d...)
		m.AllowedCIDRs = set
	}
	if m.AcknowledgePublicExposure.IsNull() || m.AcknowledgePublicExposure.IsUnknown() {
		m.AcknowledgePublicExposure = types.BoolValue(policy.AccessMode == "anywhere")
	}
}

func sameInstant(current types.String, reported string) bool {
	if current.IsNull() || current.IsUnknown() {
		return false
	}
	left, err := time.Parse(time.RFC3339, current.ValueString())
	if err != nil {
		return false
	}
	right, err := time.Parse(time.RFC3339, reported)
	return err == nil && left.Equal(right)
}
