package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ datasource.DataSource = (*projectDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*projectDataSource)(nil)

func NewProjectDataSource() datasource.DataSource { return &projectDataSource{} }

type projectDataSource struct{ client *client.Client }

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Optional: true, Computed: true},
			"name":                   schema.StringAttribute{Optional: true, Computed: true},
			"runtime":                schema.StringAttribute{Computed: true},
			"region":                 schema.StringAttribute{Computed: true},
			"port":                   schema.Int64Attribute{Computed: true},
			"description":            schema.StringAttribute{Computed: true},
			"team_id":                schema.StringAttribute{Computed: true},
			"github_repo_url":        schema.StringAttribute{Computed: true},
			"github_repo_id":         schema.Int64Attribute{Computed: true},
			"github_installation_id": schema.Int64Attribute{Computed: true},
			"github_branch":          schema.StringAttribute{Computed: true},
			"build_command":          schema.StringAttribute{Computed: true},
			"start_command":          schema.StringAttribute{Computed: true},
			"install_command":        schema.StringAttribute{Computed: true},
			"root_directory":         schema.StringAttribute{Computed: true},
			"output_directory":       schema.StringAttribute{Computed: true},
			"source_type":            schema.StringAttribute{Computed: true},
			"docker_image_uri":       schema.StringAttribute{Computed: true},
			"status":                 schema.StringAttribute{Computed: true},
			"deployment_mode":        schema.StringAttribute{Computed: true},
			"created_at":             schema.StringAttribute{Computed: true},
		},
	}
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var project *client.Project
	var err error
	if !state.ID.IsNull() && state.ID.ValueString() != "" {
		project, err = d.client.GetProject(ctx, state.ID.ValueString())
	} else {
		projects, listErr := d.client.ListProjects(ctx)
		err = listErr
		for i := range projects {
			if projects[i].Name == state.Name.ValueString() {
				project = &projects[i]
				break
			}
		}
		if err == nil && project == nil {
			err = client.ErrNotFound
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift project data source failed", err.Error())
		return
	}
	state.applyProject(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
