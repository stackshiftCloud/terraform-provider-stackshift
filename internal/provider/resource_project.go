package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stackshift/terraform-provider-stackshift/internal/provider/client"
)

var _ resource.Resource = (*projectResource)(nil)
var _ resource.ResourceWithConfigure = (*projectResource)(nil)
var _ resource.ResourceWithImportState = (*projectResource)(nil)

func NewProjectResource() resource.Resource { return &projectResource{} }

type projectResource struct{ client *client.Client }

type projectModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Runtime              types.String `tfsdk:"runtime"`
	Region               types.String `tfsdk:"region"`
	Port                 types.Int64  `tfsdk:"port"`
	Description          types.String `tfsdk:"description"`
	TeamID               types.String `tfsdk:"team_id"`
	GitHubRepoURL        types.String `tfsdk:"github_repo_url"`
	GitHubRepoID         types.Int64  `tfsdk:"github_repo_id"`
	GitHubInstallationID types.Int64  `tfsdk:"github_installation_id"`
	GitHubBranch         types.String `tfsdk:"github_branch"`
	BuildCommand         types.String `tfsdk:"build_command"`
	StartCommand         types.String `tfsdk:"start_command"`
	InstallCommand       types.String `tfsdk:"install_command"`
	RootDirectory        types.String `tfsdk:"root_directory"`
	OutputDirectory      types.String `tfsdk:"output_directory"`
	SourceType           types.String `tfsdk:"source_type"`
	DockerImageURI       types.String `tfsdk:"docker_image_uri"`
	TargetNodeID         types.String `tfsdk:"target_node_id"`
	Status               types.String `tfsdk:"status"`
	DeploymentMode       types.String `tfsdk:"deployment_mode"`
	CreatedAt            types.String `tfsdk:"created_at"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a StackShift project.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true},
			"name":                   schema.StringAttribute{Required: true},
			"runtime":                schema.StringAttribute{Required: true},
			"region":                 schema.StringAttribute{Required: true},
			"port":                   schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"description":            schema.StringAttribute{Optional: true},
			"team_id":                schema.StringAttribute{Optional: true},
			"github_repo_url":        schema.StringAttribute{Optional: true},
			"github_repo_id":         schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"github_installation_id": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"github_branch":          schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"build_command":          schema.StringAttribute{Optional: true},
			"start_command":          schema.StringAttribute{Optional: true},
			"install_command":        schema.StringAttribute{Optional: true},
			"root_directory":         schema.StringAttribute{Optional: true},
			"output_directory":       schema.StringAttribute{Optional: true},
			"source_type":            schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"docker_image_uri":       schema.StringAttribute{Optional: true},
			"target_node_id":         schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Pin the project to a registered node (BYOCloud/VPS). Omit for StackShift-managed placement.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}},
			"status":                 schema.StringAttribute{Computed: true},
			"deployment_mode":        schema.StringAttribute{Computed: true},
			"created_at":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := client.CreateProjectRequest{
		Name:                 plan.Name.ValueString(),
		Runtime:              plan.Runtime.ValueString(),
		Region:               plan.Region.ValueString(),
		Port:                 int(plan.Port.ValueInt64()),
		Description:          stringPtr(plan.Description),
		TeamID:               stringPtr(plan.TeamID),
		GitHubRepoURL:        stringPtr(plan.GitHubRepoURL),
		GitHubRepoID:         plan.GitHubRepoID.ValueInt64(),
		GitHubInstallationID: plan.GitHubInstallationID.ValueInt64(),
		GitHubBranch:         plan.GitHubBranch.ValueString(),
		BuildCommand:         stringPtr(plan.BuildCommand),
		StartCommand:         stringPtr(plan.StartCommand),
		InstallCommand:       stringPtr(plan.InstallCommand),
		RootDirectory:        stringPtr(plan.RootDirectory),
		OutputDirectory:      stringPtr(plan.OutputDirectory),
		SourceType:           plan.SourceType.ValueString(),
		DockerImageURI:       stringPtr(plan.DockerImageURI),
		TargetNodeID:         stringPtr(plan.TargetNodeID),
	}
	project, err := r.client.CreateProject(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Create StackShift project failed", err.Error())
		return
	}
	plan.applyProject(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	project, err := r.client.GetProject(ctx, state.ID.ValueString())
	if notFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read StackShift project failed", err.Error())
		return
	}
	state.applyProject(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := client.UpdateProjectRequest{
		Name:                 stringPtr(plan.Name),
		Runtime:              stringPtr(plan.Runtime),
		Region:               stringPtr(plan.Region),
		Port:                 intPtr(plan.Port),
		Description:          stringPtr(plan.Description),
		TeamID:               stringPtr(plan.TeamID),
		GitHubRepoURL:        stringPtr(plan.GitHubRepoURL),
		GitHubRepoID:         int64Ptr(plan.GitHubRepoID),
		GitHubInstallationID: int64Ptr(plan.GitHubInstallationID),
		GitHubBranch:         stringPtr(plan.GitHubBranch),
		BuildCommand:         stringPtr(plan.BuildCommand),
		StartCommand:         stringPtr(plan.StartCommand),
		InstallCommand:       stringPtr(plan.InstallCommand),
		RootDirectory:        stringPtr(plan.RootDirectory),
		OutputDirectory:      stringPtr(plan.OutputDirectory),
		SourceType:           stringPtr(plan.SourceType),
		DockerImageURI:       stringPtr(plan.DockerImageURI),
		TargetNodeID:         stringPtr(plan.TargetNodeID),
	}
	project, err := r.client.UpdateProject(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Update StackShift project failed", err.Error())
		return
	}
	plan.applyProject(project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProject(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete StackShift project failed", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (m *projectModel) applyProject(p *client.Project) {
	m.ID = types.StringValue(p.ID)
	m.Name = types.StringValue(p.Name)
	m.Runtime = types.StringValue(p.Runtime)
	m.Region = types.StringValue(p.Region)
	if p.Port != 0 {
		m.Port = types.Int64Value(int64(p.Port))
	}
	m.Description = stringValue(p.Description)
	m.TeamID = stringValue(p.TeamID)
	m.GitHubRepoURL = stringValue(p.GitHubRepoURL)
	m.GitHubRepoID = types.Int64Value(p.GitHubRepoID)
	m.GitHubInstallationID = types.Int64Value(p.GitHubInstallationID)
	m.GitHubBranch = types.StringValue(p.GitHubBranch)
	m.BuildCommand = stringValue(p.BuildCommand)
	m.StartCommand = stringValue(p.StartCommand)
	m.InstallCommand = stringValue(p.InstallCommand)
	m.RootDirectory = stringValue(p.RootDirectory)
	m.OutputDirectory = stringValue(p.OutputDirectory)
	m.SourceType = types.StringValue(p.SourceType)
	m.DockerImageURI = stringValue(p.DockerImageURI)
	m.TargetNodeID = stringValue(p.TargetNodeID)
	m.Status = types.StringValue(p.Status)
	m.DeploymentMode = types.StringValue(p.DeploymentMode)
	m.CreatedAt = timeString(p.CreatedAt)
}
