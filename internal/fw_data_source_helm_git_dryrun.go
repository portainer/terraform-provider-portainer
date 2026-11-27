package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*helmGitDryRunDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*helmGitDryRunDataSource)(nil)
)

func newHelmGitDryRunDataSource() datasource.DataSource { return &helmGitDryRunDataSource{} }

type helmGitDryRunDataSource struct{ client *APIClient }

type helmGitDryRunDataSourceModel struct {
	ID                        types.String `tfsdk:"id"`
	EndpointID                types.Int64  `tfsdk:"endpoint_id"`
	RepositoryURL             types.String `tfsdk:"repository_url"`
	ReferenceName             types.String `tfsdk:"reference_name"`
	ChartPath                 types.String `tfsdk:"chart_path"`
	ValuesFiles               types.List   `tfsdk:"values_files"`
	Namespace                 types.String `tfsdk:"namespace"`
	ReleaseName               types.String `tfsdk:"release_name"`
	RepositoryAuthentication  types.Bool   `tfsdk:"repository_authentication"`
	RepositoryUsername        types.String `tfsdk:"repository_username"`
	RepositoryPassword        types.String `tfsdk:"repository_password"`
	RepositoryGitCredentialID types.Int64  `tfsdk:"repository_git_credential_id"`
	TLSSkipVerify             types.Bool   `tfsdk:"tls_skip_verify"`
	Manifest                  types.String `tfsdk:"manifest"`
	ReleaseVersion            types.Int64  `tfsdk:"release_version"`
}

func (d *helmGitDryRunDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_helm_git_dryrun"
}

func (d *helmGitDryRunDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the dry run.",
			},
			"endpoint_id": schema.Int64Attribute{
				Required:    true,
				Description: "Environment (Endpoint) identifier",
			},
			"repository_url": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Git repository containing the Helm chart",
			},
			"reference_name": schema.StringAttribute{
				Optional:    true,
				Description: "Git reference name (e.g. refs/heads/main)",
			},
			"chart_path": schema.StringAttribute{
				Optional:    true,
				Description: "Path to the Helm chart in the repository",
			},
			"values_files": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "List of Helm values files to use",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Description: "Kubernetes namespace for the release",
			},
			"release_name": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the Helm release",
			},
			// Optional+computed so that, as with SDKv2's default, the
			// state holds false when the argument is left out.
			"repository_authentication": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the repository requires authentication",
			},
			"repository_username": schema.StringAttribute{
				Optional:    true,
				Description: "Username for repository authentication",
			},
			"repository_password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password for repository authentication",
			},
			"repository_git_credential_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Git credential ID for repository authentication",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Skip TLS verification when cloning the repository",
			},
			"manifest": schema.StringAttribute{
				Computed:    true,
				Description: "Rendered manifest from the dry run",
			},
			"release_version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version (revision) of the release",
			},
		},
	}
}

func (d *helmGitDryRunDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *helmGitDryRunDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m helmGitDryRunDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.RepositoryAuthentication.IsNull() {
		m.RepositoryAuthentication = types.BoolValue(false)
	}
	if m.TLSSkipVerify.IsNull() {
		m.TLSSkipVerify = types.BoolValue(false)
	}
	endpointID := m.EndpointID.ValueInt64()

	// Like SDKv2's GetOk, unset and zero values are left out of the payload.
	payload := map[string]interface{}{
		"repositoryURL": m.RepositoryURL.ValueString(),
	}
	if v := m.ReferenceName.ValueString(); v != "" {
		payload["repositoryReferenceName"] = v
	}
	if v := m.ChartPath.ValueString(); v != "" {
		payload["helmChartPath"] = v
	}
	if !m.ValuesFiles.IsNull() && !m.ValuesFiles.IsUnknown() && len(m.ValuesFiles.Elements()) > 0 {
		var files []string
		resp.Diagnostics.Append(m.ValuesFiles.ElementsAs(ctx, &files, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		payload["helmValuesFiles"] = files
	}
	if v := m.Namespace.ValueString(); v != "" {
		payload["namespace"] = v
	}
	if v := m.ReleaseName.ValueString(); v != "" {
		payload["name"] = v
	}
	if m.RepositoryAuthentication.ValueBool() {
		payload["repositoryAuthentication"] = true
	}
	if v := m.RepositoryUsername.ValueString(); v != "" {
		payload["repositoryUsername"] = v
	}
	if v := m.RepositoryPassword.ValueString(); v != "" {
		payload["repositoryPassword"] = v
	}
	if v := m.RepositoryGitCredentialID.ValueInt64(); v != 0 {
		payload["repositoryGitCredentialID"] = int(v)
	}
	if m.TLSSkipVerify.ValueBool() {
		payload["tlsSkipVerify"] = true
	}

	var result struct {
		Manifest  string `json:"manifest"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Version   int    `json:"version"`
	}
	if err := doJSON(ctx, d.client, "POST", fmt.Sprintf("%s/endpoints/%d/kubernetes/helm/git/dryrun", d.client.Endpoint, endpointID), payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to perform Helm Git dry run", fmt.Errorf("failed to perform Helm Git dry run: %w", err).Error())
		return
	}

	m.ID = types.StringValue(fmt.Sprintf("helm-git-dryrun-%d-%s", endpointID, m.RepositoryURL.ValueString()))
	m.Manifest = types.StringValue(result.Manifest)
	m.ReleaseVersion = types.Int64Value(int64(result.Version))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
