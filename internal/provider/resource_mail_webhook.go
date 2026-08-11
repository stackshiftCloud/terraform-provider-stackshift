package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*mailWebhookResource)(nil)
var _ resource.ResourceWithConfigure = (*mailWebhookResource)(nil)
var _ resource.ResourceWithImportState = (*mailWebhookResource)(nil)

func NewMailWebhookResource() resource.Resource { return &mailWebhookResource{} }

type mailWebhookResource struct{ client *client.Client }

type mailWebhookModel struct {
	ID            types.String `tfsdk:"id"`
	URL           types.String `tfsdk:"url"`
	Description   types.String `tfsdk:"description"`
	EventTypes    types.Set    `tfsdk:"event_types"`
	Status        types.String `tfsdk:"status"`
	Secret        types.String `tfsdk:"secret"`
	FailureCount  types.Int64  `tfsdk:"failure_count"`
	LastSuccessAt types.String `tfsdk:"last_success_at"`
	LastFailureAt types.String `tfsdk:"last_failure_at"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func (r *mailWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mail_webhook"
}

func (r *mailWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *mailWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages a signed StackShift Mail event webhook.", Attributes: map[string]schema.Attribute{
		"id":              schema.StringAttribute{Computed: true},
		"url":             schema.StringAttribute{Required: true},
		"description":     schema.StringAttribute{Optional: true},
		"event_types":     schema.SetAttribute{Required: true, ElementType: types.StringType},
		"status":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("active")},
		"secret":          schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "HMAC secret returned only at creation. Imported webhooks do not expose it."},
		"failure_count":   schema.Int64Attribute{Computed: true},
		"last_success_at": schema.StringAttribute{Computed: true},
		"last_failure_at": schema.StringAttribute{Computed: true},
		"created_at":      schema.StringAttribute{Computed: true},
	}}
}

func (r *mailWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan mailWebhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	events := []string{}
	resp.Diagnostics.Append(plan.EventTypes.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.CreateMailWebhook(
		ctx,
		plan.URL.ValueString(),
		stringPtr(plan.Description),
		events,
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Mail webhook", err.Error())
		return
	}
	secret := item.Secret
	if plan.Status.ValueString() != "active" {
		item, err = r.client.UpdateMailWebhook(
			ctx,
			item.ID,
			plan.URL.ValueString(),
			stringPtr(plan.Description),
			events,
			plan.Status.ValueString(),
		)
		if err != nil {
			resp.Diagnostics.AddError("Unable to set Mail webhook status", err.Error())
			return
		}
	}
	plan.Secret = types.StringValue(secret)
	plan.apply(ctx, item, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *mailWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state mailWebhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.GetMailWebhook(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Mail webhook", err.Error())
		return
	}
	state.apply(ctx, item, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *mailWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan mailWebhookModel
	var state mailWebhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	events := []string{}
	resp.Diagnostics.Append(plan.EventTypes.ElementsAs(ctx, &events, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	item, err := r.client.UpdateMailWebhook(
		ctx,
		state.ID.ValueString(),
		plan.URL.ValueString(),
		mailWebhookDescription(plan.Description),
		events,
		plan.Status.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Mail webhook", err.Error())
		return
	}
	plan.Secret = state.Secret
	plan.apply(ctx, item, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func mailWebhookDescription(value types.String) *string {
	description := ""
	if !value.IsNull() && !value.IsUnknown() {
		description = value.ValueString()
	}
	return &description
}

func (r *mailWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state mailWebhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteMailWebhook(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete Mail webhook", err.Error())
		}
	}
}

func (r *mailWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *mailWebhookModel) apply(ctx context.Context, item *client.MailWebhook, diagnostics interface{ AddError(string, string) }) {
	events, diags := types.SetValueFrom(ctx, types.StringType, item.EventTypes)
	if diags.HasError() {
		diagnostics.AddError("Unable to decode Mail webhook", "The API returned invalid event types.")
	}
	m.ID = types.StringValue(item.ID)
	m.URL = types.StringValue(item.URL)
	m.Description = stringValue(item.Description)
	m.EventTypes = events
	m.Status = types.StringValue(item.Status)
	m.FailureCount = types.Int64Value(item.FailureCount)
	m.LastSuccessAt = optionalTimeString(item.LastSuccessAt)
	m.LastFailureAt = optionalTimeString(item.LastFailureAt)
	m.CreatedAt = timeString(item.CreatedAt)
}
