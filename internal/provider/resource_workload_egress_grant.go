package provider

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*workloadEgressGrantResource)(nil)
var _ resource.ResourceWithConfigure = (*workloadEgressGrantResource)(nil)

func NewWorkloadEgressGrantResource() resource.Resource {
	return &workloadEgressGrantResource{}
}

type workloadEgressGrantResource struct{ client *client.Client }

type workloadEgressGrantModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	DependencyID      types.String `tfsdk:"dependency_id"`
	Environment       types.String `tfsdk:"environment"`
	Protocol          types.String `tfsdk:"protocol"`
	Hostname          types.String `tfsdk:"hostname"`
	CIDR              types.String `tfsdk:"cidr"`
	PortStart         types.Int64  `tfsdk:"port_start"`
	PortEnd           types.Int64  `tfsdk:"port_end"`
	Purpose           types.String `tfsdk:"purpose"`
	BroadPublicEgress types.Bool   `tfsdk:"broad_public_egress"`
	ExpiresAt         types.String `tfsdk:"expires_at"`
	Status            types.String `tfsdk:"status"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	IdempotencyKey    types.String `tfsdk:"idempotency_key"`
}

func (r *workloadEgressGrantResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_workload_egress_grant"
}

func (r *workloadEgressGrantResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *workloadEgressGrantResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	stringReplace := replaceString()
	intReplace := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Requests a scoped, expiring hosted-workload egress grant. StackShift operator approval is always separate.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"project_id":    schema.StringAttribute{Required: true, PlanModifiers: stringReplace},
			"dependency_id": schema.StringAttribute{Optional: true, PlanModifiers: stringReplace},
			"environment": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("production"),
				PlanModifiers: stringReplace,
			},
			"protocol": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: append(replaceString(), stringplanmodifier.UseStateForUnknown()),
			},
			"hostname": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: append(replaceString(), stringplanmodifier.UseStateForUnknown()),
			},
			"cidr": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers: append(stringReplace, stringplanmodifier.UseStateForUnknown()),
			},
			"port_start": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: intReplace},
			"port_end":   schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: intReplace},
			"purpose":    schema.StringAttribute{Required: true, PlanModifiers: stringReplace},
			"broad_public_egress": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"expires_at": schema.StringAttribute{
				Required: true, PlanModifiers: replaceString(),
				MarkdownDescription: "RFC3339 expiry. Destination grants may last up to 90 days; broad public grants up to seven days.",
			},
			"status":     schema.StringAttribute{Computed: true},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
			"idempotency_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       stringReplace,
				MarkdownDescription: "Stable create key. When omitted, Terraform derives it from the immutable grant request.",
			},
		},
	}
}

func (r *workloadEgressGrantResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan workloadEgressGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	idempotencyKey := workloadEgressGrantIdempotencyKey(plan)
	grant, err := r.client.RequestWorkloadEgressGrant(ctx, plan.ProjectID.ValueString(), idempotencyKey, client.RequestWorkloadEgressGrant{
		DependencyID: stringPtr(plan.DependencyID), Environment: canonicalWorkloadValue(plan.Environment.ValueString()),
		Protocol: canonicalWorkloadValue(plan.Protocol.ValueString()), Hostname: canonicalHostnamePointer(plan.Hostname), CIDR: stringPtr(plan.CIDR),
		PortStart: plan.PortStart.ValueInt64(), PortEnd: plan.PortEnd.ValueInt64(),
		Purpose: plan.Purpose.ValueString(), BroadPublicEgress: plan.BroadPublicEgress.ValueBool(),
		ExpiresAt: canonicalRFC3339(plan.ExpiresAt.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Request workload egress grant failed", err.Error())
		return
	}
	plan.IdempotencyKey = types.StringValue(idempotencyKey)
	plan.apply(*grant)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func workloadEgressGrantIdempotencyKey(plan workloadEgressGrantModel) string {
	if configured := strings.TrimSpace(plan.IdempotencyKey.ValueString()); configured != "" {
		return configured
	}
	return deterministicIdempotencyKey(
		"tf-workload-egress-grant-create",
		plan.ProjectID.ValueString(),
		plan.DependencyID.ValueString(),
		plan.Environment.ValueString(),
		plan.Protocol.ValueString(),
		plan.Hostname.ValueString(),
		plan.CIDR.ValueString(),
		strconv.FormatInt(plan.PortStart.ValueInt64(), 10),
		strconv.FormatInt(plan.PortEnd.ValueInt64(), 10),
		plan.Purpose.ValueString(),
		strconv.FormatBool(plan.BroadPublicEgress.ValueBool()),
		canonicalRFC3339(plan.ExpiresAt.ValueString()),
	)
}

func canonicalRFC3339(value string) string {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return parsed.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
}

func (r *workloadEgressGrantResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state workloadEgressGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	grants, err := r.client.ListWorkloadEgressGrants(ctx, state.ProjectID.ValueString(), canonicalWorkloadValue(state.Environment.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Read workload egress grant failed", err.Error())
		return
	}
	for _, grant := range grants {
		if grant.ID == state.ID.ValueString() {
			state.apply(grant)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *workloadEgressGrantResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan workloadEgressGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workloadEgressGrantResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state workloadEgressGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RevokeWorkloadEgressGrant(
		ctx, state.ProjectID.ValueString(), state.ID.ValueString(),
	); err != nil {
		resp.Diagnostics.AddError("Revoke workload egress grant failed", err.Error())
	}
}

func (m *workloadEgressGrantModel) apply(grant client.WorkloadEgressGrant) {
	m.ID = types.StringValue(grant.ID)
	m.ProjectID = types.StringValue(grant.ProjectID)
	m.DependencyID = stringValue(grant.DependencyID)
	m.Environment = preserveWorkloadString(m.Environment, grant.Environment, canonicalWorkloadValue)
	m.Protocol = preserveWorkloadString(m.Protocol, grant.Protocol, canonicalWorkloadValue)
	if grant.Hostname == nil {
		m.Hostname = types.StringNull()
	} else {
		m.Hostname = preserveWorkloadString(m.Hostname, *grant.Hostname, canonicalWorkloadHostname)
	}
	m.CIDR = stringValue(grant.CIDR)
	m.PortStart = types.Int64Value(grant.PortStart)
	m.PortEnd = types.Int64Value(grant.PortEnd)
	m.Purpose = preserveWorkloadString(m.Purpose, grant.Purpose, strings.TrimSpace)
	m.BroadPublicEgress = types.BoolValue(grant.BroadPublicEgress)
	m.ExpiresAt = preserveWorkloadString(m.ExpiresAt, grant.ExpiresAt, canonicalRFC3339)
	m.Status = types.StringValue(grant.Status)
	m.CreatedAt = types.StringValue(grant.CreatedAt)
	m.UpdatedAt = types.StringValue(grant.UpdatedAt)
}

func canonicalHostnamePointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	hostname := canonicalWorkloadHostname(value.ValueString())
	return &hostname
}
