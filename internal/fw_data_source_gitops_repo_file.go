package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsRepoFileDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsRepoFileDataSource)(nil)
)

func newGitopsRepoFileDataSource() datasource.DataSource { return &gitopsRepoFileDataSource{} }

type gitopsRepoFileDataSource struct{ client *APIClient }

type gitopsRepoFileModel struct {
	ID              types.String `tfsdk:"id"`
	RepositoryURL   types.String `tfsdk:"repository_url"`
	Reference       types.String `tfsdk:"reference"`
	TargetFile      types.String `tfsdk:"target_file"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	GitCredentialID types.Int64  `tfsdk:"git_credential_id"`
	TLSSkipVerify   types.Bool   `tfsdk:"tls_skip_verify"`
	FileContent     types.String `tfsdk:"file_content"`
}

func (d *gitopsRepoFileDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_repo_file"
}

func (d *gitopsRepoFileDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"repository_url": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Git repository",
			},
			"reference": schema.StringAttribute{
				Optional:    true,
				Description: "Git reference (e.g. refs/heads/master)",
			},
			"target_file": schema.StringAttribute{
				Optional:    true,
				Description: "Path to the file whose content will be read",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username for repository authentication",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password for repository authentication",
			},
			"git_credential_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Git credential ID for authentication",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Skip TLS verification when cloning the Git repository",
			},
			"file_content": schema.StringAttribute{
				Computed:    true,
				Description: "Content of the file from the Git repository",
			},
		},
	}
}

func (d *gitopsRepoFileDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsRepoFileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsRepoFileModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	repoURL := data.RepositoryURL.ValueString()
	data.TLSSkipVerify = gitopsDSBoolDefault(data.TLSSkipVerify)

	payload := map[string]interface{}{
		"repository": repoURL,
	}
	if v, ok := gitopsDSString(data.Reference); ok {
		payload["reference"] = v
	}
	if v, ok := gitopsDSString(data.TargetFile); ok {
		payload["targetFile"] = v
	}
	if v, ok := gitopsDSString(data.Username); ok {
		payload["username"] = v
	}
	if v, ok := gitopsDSString(data.Password); ok {
		payload["password"] = v
	}
	if v, ok := gitopsDSInt(data.GitCredentialID); ok {
		payload["gitCredentialID"] = v
	}
	if data.TLSSkipVerify.ValueBool() {
		payload["TLSSkipVerify"] = true
	}

	var result struct {
		FileContent string `json:"FileContent"`
	}
	if err := doJSON(ctx, client, "POST", client.Endpoint+"/gitops/repo/file/preview", payload, &result); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to preview Git repository file: %w", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("gitops-repo-file-%s-%s", repoURL, data.TargetFile.ValueString()))
	data.FileContent = types.StringValue(result.FileContent)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
