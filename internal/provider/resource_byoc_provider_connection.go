package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*byocProviderConnectionResource)(nil)
var _ resource.ResourceWithConfigure = (*byocProviderConnectionResource)(nil)
var _ resource.ResourceWithImportState = (*byocProviderConnectionResource)(nil)

func NewBYOCProviderConnectionResource() resource.Resource {
	return &byocProviderConnectionResource{}
}

type byocProviderConnectionResource struct{ client *client.Client }

type byocProviderConnectionModel struct {
	ID                  types.String `tfsdk:"id"`
	Provider            types.String `tfsdk:"provider"`
	DisplayName         types.String `tfsdk:"display_name"`
	Token               types.String `tfsdk:"token"`
	RoleARN             types.String `tfsdk:"role_arn"`
	Region              types.String `tfsdk:"region"`
	AzureTenantID       types.String `tfsdk:"azure_tenant_id"`
	AzureSubscriptionID types.String `tfsdk:"azure_subscription_id"`
	AzureClientID       types.String `tfsdk:"azure_client_id"`
	ValidateOnApply     types.Bool   `tfsdk:"validate_on_apply"`
	AuthType            types.String `tfsdk:"auth_type"`
	ExternalID          types.String `tfsdk:"external_id"`
	Status              types.String `tfsdk:"status"`
	ValidationError     types.String `tfsdk:"validation_error"`
	MissingPermissions  types.List   `tfsdk:"missing_permissions"`
}

func (r *byocProviderConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_byoc_provider_connection"
}

func (r *byocProviderConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
	}
}

func (r *byocProviderConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a clean-contract BYOCloud provider connection. Hetzner and DigitalOcean accept scoped tokens; AWS accepts only an assumed-role ARN; Azure accepts only workload-federation identity fields.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.StringAttribute{Computed: true},
			"provider":              schema.StringAttribute{Required: true, PlanModifiers: replace},
			"display_name":          schema.StringAttribute{Required: true},
			"token":                 schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Scoped Hetzner or DigitalOcean token. Never use this attribute for AWS or Azure."},
			"role_arn":              schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "AWS IAM role ARN assumed by StackShift using the computed external_id."},
			"region":                schema.StringAttribute{Optional: true, MarkdownDescription: "Default AWS region."},
			"azure_tenant_id":       schema.StringAttribute{Optional: true},
			"azure_subscription_id": schema.StringAttribute{Optional: true},
			"azure_client_id":       schema.StringAttribute{Optional: true},
			"validate_on_apply":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"auth_type":             schema.StringAttribute{Computed: true},
			"external_id":           schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Server-generated AWS external ID used in the role trust policy."},
			"status":                schema.StringAttribute{Computed: true},
			"validation_error":      schema.StringAttribute{Computed: true},
			"missing_permissions":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
}

func (m byocProviderConnectionModel) request() (client.BYOCProviderConnectionRequest, error) {
	provider := strings.ToLower(strings.TrimSpace(m.Provider.ValueString()))
	credentials := client.BYOCProviderConnectionCredential{
		Token: strings.TrimSpace(m.Token.ValueString()), RoleARN: strings.TrimSpace(m.RoleARN.ValueString()), Region: strings.TrimSpace(m.Region.ValueString()),
		AzureTenantID: strings.TrimSpace(m.AzureTenantID.ValueString()), AzureSubscriptionID: strings.TrimSpace(m.AzureSubscriptionID.ValueString()), AzureClientID: strings.TrimSpace(m.AzureClientID.ValueString()),
	}
	switch provider {
	case "hetzner", "digitalocean":
		if credentials.Token == "" || credentials.RoleARN != "" || credentials.AzureTenantID != "" || credentials.AzureSubscriptionID != "" || credentials.AzureClientID != "" {
			return client.BYOCProviderConnectionRequest{}, fmt.Errorf("%s requires exactly one scoped token and no cloud-federation fields", provider)
		}
	case "aws":
		if credentials.RoleARN == "" || credentials.Region == "" || credentials.Token != "" || credentials.AzureTenantID != "" || credentials.AzureSubscriptionID != "" || credentials.AzureClientID != "" {
			return client.BYOCProviderConnectionRequest{}, errors.New("aws requires role_arn and region; access keys, tokens, and Azure identity fields are forbidden")
		}
	case "azure":
		if credentials.AzureTenantID == "" || credentials.AzureSubscriptionID == "" || credentials.AzureClientID == "" || credentials.Token != "" || credentials.RoleARN != "" {
			return client.BYOCProviderConnectionRequest{}, errors.New("azure requires tenant, subscription, and client IDs; tokens, client secrets, and AWS role fields are forbidden")
		}
	default:
		return client.BYOCProviderConnectionRequest{}, fmt.Errorf("unsupported BYOCloud provider %q", provider)
	}
	return client.BYOCProviderConnectionRequest{Provider: provider, DisplayName: strings.TrimSpace(m.DisplayName.ValueString()), Credentials: credentials}, nil
}

func (r *byocProviderConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan byocProviderConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request, err := plan.request()
	if err != nil {
		resp.Diagnostics.AddError("Invalid BYOCloud provider connection", err.Error())
		return
	}
	connection, err := r.client.CreateBYOCProviderConnection(ctx, request)
	if err == nil && plan.ValidateOnApply.ValueBool() {
		connection, err = r.client.ValidateBYOCProviderConnection(ctx, connection.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create BYOCloud provider connection", err.Error())
		return
	}
	plan.apply(ctx, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocProviderConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state byocProviderConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	connection, err := r.client.GetBYOCProviderConnection(ctx, state.ID.ValueString())
	if errors.Is(err, client.ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read BYOCloud provider connection", err.Error())
		return
	}
	state.apply(ctx, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *byocProviderConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan byocProviderConnectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	request, err := plan.request()
	if err != nil {
		resp.Diagnostics.AddError("Invalid BYOCloud provider connection", err.Error())
		return
	}
	connection, err := r.client.UpdateBYOCProviderConnection(ctx, plan.ID.ValueString(), request)
	if err == nil && plan.ValidateOnApply.ValueBool() {
		connection, err = r.client.ValidateBYOCProviderConnection(ctx, connection.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update BYOCloud provider connection", err.Error())
		return
	}
	plan.apply(ctx, connection)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *byocProviderConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state byocProviderConnectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if !resp.Diagnostics.HasError() {
		if err := r.client.DeleteBYOCProviderConnection(ctx, state.ID.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to delete BYOCloud provider connection", err.Error())
		}
	}
}

func (r *byocProviderConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *byocProviderConnectionModel) apply(ctx context.Context, connection *client.BYOCProviderConnection) {
	// Preserve secret input attributes while refreshing only server-owned fields.
	m.ID = types.StringValue(connection.ID)
	m.Provider = types.StringValue(connection.Provider)
	m.DisplayName = types.StringValue(connection.DisplayName)
	m.AuthType = types.StringValue(connection.AuthType)
	m.ExternalID = optionalBYOCString(connection.ExternalID)
	m.Status = types.StringValue(connection.Status)
	m.ValidationError = optionalBYOCString(connection.ValidationError)
	permissions, _ := types.ListValueFrom(ctx, types.StringType, connection.ValidationDetails.MissingPermissions)
	m.MissingPermissions = permissions
}

func optionalBYOCString(value string) types.String {
	if strings.TrimSpace(value) == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
