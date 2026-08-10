package provider

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*assetWebhookResource)(nil)
var _ resource.ResourceWithConfigure = (*assetWebhookResource)(nil)
var _ resource.ResourceWithImportState = (*assetWebhookResource)(nil)

func NewAssetWebhookResource() resource.Resource { return &assetWebhookResource{} }

type assetWebhookResource struct{ client *client.Client }

type assetWebhookModel struct {
	ID            types.String `tfsdk:"id"`
	URL           types.String `tfsdk:"url"`
	EventTypes    types.Set    `tfsdk:"event_types"`
	Secret        types.String `tfsdk:"secret"`
	Status        types.String `tfsdk:"status"`
	FailureCount  types.Int64  `tfsdk:"failure_count"`
	LastSuccessAt types.String `tfsdk:"last_success_at"`
	LastFailureAt types.String `tfsdk:"last_failure_at"`
	DisabledAt    types.String `tfsdk:"disabled_at"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *assetWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_webhook"
}

func (r *assetWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *assetWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a signed StackShift Assets webhook subscription.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true},
		"url":             schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"event_types":     schema.SetAttribute{Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()}},
		"secret":          schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "HMAC signing secret returned only when the webhook is created."},
		"status":          schema.StringAttribute{Computed: true},
		"failure_count":   schema.Int64Attribute{Computed: true},
		"last_success_at": schema.StringAttribute{Computed: true},
		"last_failure_at": schema.StringAttribute{Computed: true},
		"disabled_at":     schema.StringAttribute{Computed: true},
		"created_at":      schema.StringAttribute{Computed: true},
		"updated_at":      schema.StringAttribute{Computed: true},
	}}
}

func (r *assetWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan assetWebhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var events []string
	resp.Diagnostics.Append(plan.EventTypes.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	webhook, secret, err := r.client.CreateAssetWebhook(ctx, plan.URL.ValueString(), events)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Assets webhook", err.Error())
		return
	}
	plan.Secret = types.StringValue(secret)
	plan.apply(ctx, webhook, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state assetWebhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	webhook, err := r.client.GetAssetWebhook(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Assets webhook", err.Error())
		return
	}
	state.apply(ctx, webhook, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *assetWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan assetWebhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *assetWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state assetWebhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAssetWebhook(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete Assets webhook", err.Error())
	}
}

func (r *assetWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *assetWebhookModel) apply(ctx context.Context, webhook *client.AssetWebhook, diagnostics interface{ AddError(string, string) }) {
	events, diags := types.SetValueFrom(ctx, types.StringType, webhook.EventTypes)
	if diags.HasError() {
		diagnostics.AddError("Unable to decode Assets webhook", "The API returned invalid event types")
	}
	m.ID = types.StringValue(webhook.ID)
	m.URL = types.StringValue(webhook.URL)
	m.EventTypes = events
	m.Status = types.StringValue(webhook.Status)
	m.FailureCount = types.Int64Value(webhook.FailureCount)
	m.LastSuccessAt = optionalAssetWebhookTime(webhook.LastSuccessAt)
	m.LastFailureAt = optionalAssetWebhookTime(webhook.LastFailureAt)
	m.DisabledAt = optionalAssetWebhookTime(webhook.DisabledAt)
	m.CreatedAt = types.StringValue(webhook.CreatedAt.UTC().Format(time.RFC3339))
	m.UpdatedAt = types.StringValue(webhook.UpdatedAt.UTC().Format(time.RFC3339))
}

func optionalAssetWebhookTime(value *time.Time) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.UTC().Format(time.RFC3339))
}
