package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

type computeInstanceResource struct{ client *client.Client }
type computeActionResource struct{ client *client.Client }

type computeInstanceModel struct {
	ID                     types.String `tfsdk:"id"`
	PlanID                 types.String `tfsdk:"plan_id"`
	PlanSlug               types.String `tfsdk:"plan_slug"`
	Name                   types.String `tfsdk:"name"`
	Hostname               types.String `tfsdk:"hostname"`
	ImageSlug              types.String `tfsdk:"image_slug"`
	SSHPublicKey           types.String `tfsdk:"ssh_public_key"`
	Mode                   types.String `tfsdk:"mode"`
	InstallDocker          types.Bool   `tfsdk:"install_docker"`
	InstallStackShiftAgent types.Bool   `tfsdk:"install_stackshift_agent"`
	IdempotencyKey         types.String `tfsdk:"idempotency_key"`
	PaymentProvider        types.String `tfsdk:"payment_provider"`
	Status                 types.String `tfsdk:"status"`
	IPv4Address            types.String `tfsdk:"ipv4_address"`
	IPv6Address            types.String `tfsdk:"ipv6_address"`
	OperationID            types.String `tfsdk:"operation_id"`
	PaymentURL             types.String `tfsdk:"payment_url"`
	PaymentReference       types.String `tfsdk:"payment_reference"`
}

type computeActionModel struct {
	ID           types.String `tfsdk:"id"`
	InstanceID   types.String `tfsdk:"instance_id"`
	Action       types.String `tfsdk:"action"`
	Confirmation types.String `tfsdk:"confirmation"`
	ImageSlug    types.String `tfsdk:"image_slug"`
	SSHPublicKey types.String `tfsdk:"ssh_public_key"`
	Hostname     types.String `tfsdk:"hostname"`
	Message      types.String `tfsdk:"message"`
	Nonce        types.String `tfsdk:"nonce"`
}

func NewComputeInstanceResource() resource.Resource { return &computeInstanceResource{} }
func NewComputeActionResource() resource.Resource   { return &computeActionResource{} }

func (r *computeInstanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}
func (r *computeInstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *computeInstanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":                       schema.StringAttribute{Computed: true},
		"plan_id":                  schema.StringAttribute{Optional: true, Computed: true},
		"plan_slug":                schema.StringAttribute{Optional: true},
		"name":                     schema.StringAttribute{Required: true},
		"hostname":                 schema.StringAttribute{Required: true},
		"image_slug":               schema.StringAttribute{Required: true},
		"ssh_public_key":           schema.StringAttribute{Required: true, Sensitive: true},
		"mode":                     schema.StringAttribute{Required: true},
		"install_docker":           schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"install_stackshift_agent": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		"idempotency_key":          schema.StringAttribute{Optional: true},
		"payment_provider":         schema.StringAttribute{Optional: true},
		"status":                   schema.StringAttribute{Computed: true},
		"ipv4_address":             schema.StringAttribute{Computed: true},
		"ipv6_address":             schema.StringAttribute{Computed: true},
		"operation_id":             schema.StringAttribute{Computed: true},
		"payment_url":              schema.StringAttribute{Computed: true, Sensitive: true},
		"payment_reference":        schema.StringAttribute{Computed: true, Sensitive: true},
	}}
}
func (r *computeInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan computeInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	instance, result, err := r.client.CreateComputeInstance(ctx, client.CreateComputeInstanceRequest{
		PlanID: stringPtr(plan.PlanID), PlanSlug: plan.PlanSlug.ValueString(), Name: plan.Name.ValueString(),
		Hostname: plan.Hostname.ValueString(), ImageSlug: plan.ImageSlug.ValueString(), SSHPublicKey: plan.SSHPublicKey.ValueString(),
		Mode: plan.Mode.ValueString(), InstallDocker: plan.InstallDocker.ValueBool(), InstallStackShiftAgent: plan.InstallStackShiftAgent.ValueBool(),
		IdempotencyKey: plan.IdempotencyKey.ValueString(), PaymentProvider: plan.PaymentProvider.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Create compute instance failed", err.Error())
		return
	}
	plan.applyCompute(instance)
	plan.OperationID = types.StringValue(result.OperationID)
	plan.PaymentURL = types.StringValue(result.PaymentURL)
	plan.PaymentReference = types.StringValue(result.PaymentRef)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *computeInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state computeInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	instance, err := r.client.GetComputeInstance(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read compute instance failed", err.Error())
		return
	}
	state.applyCompute(instance)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *computeInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan computeInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *computeInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state computeInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		_, err := r.client.ComputeAction(ctx, state.ID.ValueString(), client.ComputeActionRequest{Action: "delete", Confirmation: "delete"})
		if err != nil {
			resp.Diagnostics.AddError("Delete compute instance failed", err.Error())
		}
	}
}
func (m *computeInstanceModel) applyCompute(i *client.ComputeInstance) {
	m.ID = types.StringValue(i.ID)
	m.PlanID = types.StringValue(i.PlanID)
	m.Name = types.StringValue(i.Name)
	m.Hostname = types.StringValue(i.Hostname)
	m.ImageSlug = types.StringValue(i.ImageSlug)
	m.Mode = types.StringValue(i.Mode)
	m.InstallDocker = types.BoolValue(i.InstallDocker)
	m.InstallStackShiftAgent = types.BoolValue(i.InstallStackShiftAgent)
	m.Status = types.StringValue(i.Status)
	m.IPv4Address = stringValue(i.IPv4Address)
	m.IPv6Address = stringValue(i.IPv6Address)
}

func (r *computeActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_action"
}
func (r *computeActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}
func (r *computeActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "instance_id": schema.StringAttribute{Required: true, PlanModifiers: replaceString()},
		"action": schema.StringAttribute{Required: true, PlanModifiers: replaceString()}, "confirmation": schema.StringAttribute{Optional: true},
		"image_slug": schema.StringAttribute{Optional: true}, "ssh_public_key": schema.StringAttribute{Optional: true, Sensitive: true},
		"hostname": schema.StringAttribute{Optional: true}, "message": schema.StringAttribute{Computed: true}, "nonce": schema.StringAttribute{Optional: true, PlanModifiers: replaceString()},
	}}
}
func (r *computeActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan computeActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.ComputeAction(ctx, plan.InstanceID.ValueString(), client.ComputeActionRequest{
		Action: plan.Action.ValueString(), Confirmation: plan.Confirmation.ValueString(), ImageSlug: plan.ImageSlug.ValueString(),
		SSHPublicKey: plan.SSHPublicKey.ValueString(), Hostname: plan.Hostname.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Compute action failed", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.Action.ValueString() + ":" + time.Now().UTC().Format(time.RFC3339Nano))
	plan.Message = types.StringValue(result.Message)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *computeActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state computeActionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *computeActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan computeActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *computeActionResource) Delete(ctx context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
