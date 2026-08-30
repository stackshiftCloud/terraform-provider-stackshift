package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*workloadDependencyResource)(nil)
var _ resource.ResourceWithConfigure = (*workloadDependencyResource)(nil)

func NewWorkloadDependencyResource() resource.Resource {
	return &workloadDependencyResource{}
}

type workloadDependencyResource struct{ client *client.Client }

type workloadDependencyModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Environment          types.String `tfsdk:"environment"`
	Hostname             types.String `tfsdk:"hostname"`
	Port                 types.Int64  `tfsdk:"port"`
	Protocol             types.String `tfsdk:"protocol"`
	Purpose              types.String `tfsdk:"purpose"`
	TLSRequired          types.Bool   `tfsdk:"tls_required"`
	PrivateRouteRequired types.Bool   `tfsdk:"private_route_required"`
	Detected             types.Bool   `tfsdk:"detected"`
	Status               types.String `tfsdk:"status"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func (r *workloadDependencyResource) Metadata(
	_ context.Context,
	req resource.MetadataRequest,
	resp *resource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_workload_external_dependency"
}

func (r *workloadDependencyResource) Configure(
	_ context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *workloadDependencyResource) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	resp *resource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Declares an exact external service dependency for a hosted StackShift workload. Declaration does not approve network access.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"project_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"environment": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("production"),
				PlanModifiers: replaceString(),
			},
			"hostname": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"port": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"protocol": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("tls"),
				PlanModifiers: replaceString(),
			},
			"purpose": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
			"tls_required": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"private_route_required": schema.BoolAttribute{Computed: true},
			"detected":               schema.BoolAttribute{Computed: true},
			"status":                 schema.StringAttribute{Computed: true},
			"created_at":             schema.StringAttribute{Computed: true},
			"updated_at":             schema.StringAttribute{Computed: true},
		},
	}
}

func (r *workloadDependencyResource) Create(
	ctx context.Context,
	req resource.CreateRequest,
	resp *resource.CreateResponse,
) {
	var plan workloadDependencyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dependencies, err := r.client.DeclareWorkloadDependency(ctx, plan.ProjectID.ValueString(), client.DeclareWorkloadDependencyRequest{
		Environment: canonicalWorkloadValue(plan.Environment.ValueString()), Hostname: canonicalWorkloadHostname(plan.Hostname.ValueString()),
		Port: plan.Port.ValueInt64(), Protocol: canonicalWorkloadValue(plan.Protocol.ValueString()),
		Purpose: plan.Purpose.ValueString(), TLSRequired: plan.TLSRequired.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Declare workload dependency failed", err.Error())
		return
	}
	dependency := matchingDependency(dependencies, plan)
	if dependency == nil {
		resp.Diagnostics.AddError("Declare workload dependency failed", "StackShift did not return the declared dependency.")
		return
	}
	plan.apply(*dependency)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workloadDependencyResource) Read(
	ctx context.Context,
	req resource.ReadRequest,
	resp *resource.ReadResponse,
) {
	var state workloadDependencyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dependencies, err := r.client.ListWorkloadDependencies(ctx, state.ProjectID.ValueString(), canonicalWorkloadValue(state.Environment.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Read workload dependency failed", err.Error())
		return
	}
	for _, dependency := range dependencies {
		if dependency.ID == state.ID.ValueString() {
			state.apply(dependency)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *workloadDependencyResource) Update(
	ctx context.Context,
	req resource.UpdateRequest,
	resp *resource.UpdateResponse,
) {
	var plan workloadDependencyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workloadDependencyResource) Delete(
	ctx context.Context,
	req resource.DeleteRequest,
	resp *resource.DeleteResponse,
) {
	var state workloadDependencyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DisableWorkloadDependency(
		ctx, state.ProjectID.ValueString(), state.ID.ValueString(),
	); err != nil {
		resp.Diagnostics.AddError("Disable workload dependency failed", err.Error())
	}
}

func matchingDependency(
	dependencies []client.WorkloadExternalDependency,
	plan workloadDependencyModel,
) *client.WorkloadExternalDependency {
	for index := range dependencies {
		dependency := &dependencies[index]
		if canonicalWorkloadValue(dependency.Environment) == canonicalWorkloadValue(plan.Environment.ValueString()) &&
			canonicalWorkloadHostname(dependency.Hostname) == canonicalWorkloadHostname(plan.Hostname.ValueString()) &&
			dependency.Port == plan.Port.ValueInt64() &&
			canonicalWorkloadValue(dependency.Protocol) == canonicalWorkloadValue(plan.Protocol.ValueString()) {
			return dependency
		}
	}
	return nil
}

func canonicalWorkloadValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func canonicalWorkloadHostname(value string) string {
	return strings.TrimSuffix(canonicalWorkloadValue(value), ".")
}

func (m *workloadDependencyModel) apply(dependency client.WorkloadExternalDependency) {
	m.ID = types.StringValue(dependency.ID)
	m.ProjectID = types.StringValue(dependency.ProjectID)
	m.Environment = preserveWorkloadString(m.Environment, dependency.Environment, canonicalWorkloadValue)
	m.Hostname = preserveWorkloadString(m.Hostname, dependency.Hostname, canonicalWorkloadHostname)
	m.Port = types.Int64Value(dependency.Port)
	m.Protocol = preserveWorkloadString(m.Protocol, dependency.Protocol, canonicalWorkloadValue)
	m.Purpose = preserveWorkloadString(m.Purpose, dependency.Purpose, strings.TrimSpace)
	m.TLSRequired = types.BoolValue(dependency.TLSRequired)
	m.PrivateRouteRequired = types.BoolValue(dependency.PrivateRouteRequired)
	m.Detected = types.BoolValue(dependency.Detected)
	m.Status = types.StringValue(dependency.Status)
	m.CreatedAt = types.StringValue(dependency.CreatedAt)
	m.UpdatedAt = types.StringValue(dependency.UpdatedAt)
}

func preserveWorkloadString(current types.String, remote string, canonicalize func(string) string) types.String {
	if !current.IsNull() && !current.IsUnknown() && canonicalize(current.ValueString()) == canonicalize(remote) {
		return current
	}
	return types.StringValue(remote)
}
