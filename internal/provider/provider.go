package provider

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

const defaultEndpoint = "https://api.stackshift.cloud"

var _ provider.Provider = (*stackshiftProvider)(nil)

func New() provider.Provider {
	return &stackshiftProvider{}
}

type stackshiftProvider struct{}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIToken types.String `tfsdk:"api_token"`
}

func (p *stackshiftProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stackshift"
}

func (p *stackshiftProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform/OpenTofu provider for StackShift.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "StackShift API endpoint. Defaults to https://api.stackshift.cloud. Can also be set with STACKSHIFT_ENDPOINT.",
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "StackShift API token with the `sspat_` prefix. Can also be set with STACKSHIFT_API_TOKEN.",
			},
		},
	}
}

func (p *stackshiftProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := strings.TrimSpace(os.Getenv("STACKSHIFT_ENDPOINT"))
	if !cfg.Endpoint.IsNull() && !cfg.Endpoint.IsUnknown() {
		endpoint = strings.TrimSpace(cfg.Endpoint.ValueString())
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	token := strings.TrimSpace(os.Getenv("STACKSHIFT_API_TOKEN"))
	if !cfg.APIToken.IsNull() && !cfg.APIToken.IsUnknown() {
		token = strings.TrimSpace(cfg.APIToken.ValueString())
	}
	if token == "" {
		resp.Diagnostics.AddError("Missing StackShift API token", "Set api_token in provider configuration or STACKSHIFT_API_TOKEN in the environment.")
		return
	}

	c := client.New(endpoint, token, &http.Client{Timeout: 30 * time.Second})
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *stackshiftProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewProjectEnvResource,
		NewDatabaseResource,
		NewBucketResource,
		NewDomainResource,
		NewDNSRecordResource,
		NewBuildActionResource,
		NewDeploymentActionResource,
		NewComputeInstanceResource,
		NewComputeActionResource,
		NewBYOCProviderConnectionResource,
		NewBYOCNodeResource,
		NewBYOCVolumeResource,
		NewBYOCSnapshotResource,
		NewBYOCStaticIPResource,
		NewAgencyClientResource,
		NewAgencyResourceAssignmentResource,
		NewRunbookResource,
		NewRunbookExecutionResource,
		NewAssetBucketResource,
		NewAssetWebhookResource,
		NewAssetLifecycleRuleResource,
		NewAssetDomainResource,
		NewAssetTransformationResource,
		NewAssetContentPolicyResource,
		NewMailDomainResource,
		NewMailWebhookResource,
	}
}

func (p *stackshiftProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewDatabaseDataSource,
		NewDatabaseCredentialsDataSource,
		NewAssetDataSource,
		NewAssetJobDataSource,
		NewAssetAnalyticsDataSource,
	}
}
