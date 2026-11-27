package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsRepoFileSearchDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsRepoFileSearchDataSource)(nil)
)

func newGitopsRepoFileSearchDataSource() datasource.DataSource {
	return &gitopsRepoFileSearchDataSource{}
}

type gitopsRepoFileSearchDataSource struct{ client *APIClient }

type gitopsRepoFileSearchModel struct {
	ID              types.String `tfsdk:"id"`
	Repository      types.String `tfsdk:"repository"`
	Reference       types.String `tfsdk:"reference"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	SourceID        types.Int64  `tfsdk:"source_id"`
	TLSSkipVerify   types.Bool   `tfsdk:"tls_skip_verify"`
	Keyword         types.String `tfsdk:"keyword"`
	Include         types.String `tfsdk:"include"`
	DirectoriesOnly types.Bool   `tfsdk:"directories_only"`
	Force           types.Bool   `tfsdk:"force"`
	Paths           types.List   `tfsdk:"paths"`
}

func (d *gitopsRepoFileSearchDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_repo_file_search"
}

func (d *gitopsRepoFileSearchDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"repository": schema.StringAttribute{
				Required:    true,
				Description: "URL of the git repository to search.",
			},
			"reference": schema.StringAttribute{
				Required:    true,
				Description: "Git reference to search at, for example `refs/heads/main`.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username to authenticate to the repository with. Leave unset for a public repository or when `source_id` supplies the credentials.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password or token to authenticate with. Stored in state as a sensitive value.",
			},
			"source_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of a GitOps source to take the credentials from instead of giving them here.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to skip verification of the git server's TLS certificate.",
			},
			"keyword": schema.StringAttribute{
				Optional:    true,
				Description: "Only return paths matching this keyword.",
			},
			"include": schema.StringAttribute{
				Optional:    true,
				Description: "File extensions to include, for example `yml,yaml`.",
			},
			"directories_only": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to return only directories rather than files.",
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to re-clone the repository rather than answer from Portainer's cache.",
			},
			"paths": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The paths the search matched, sorted.",
			},
		},
	}
}

func (d *gitopsRepoFileSearchDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsRepoFileSearchDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsRepoFileSearchModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	data.TLSSkipVerify = gitopsDSBoolDefault(data.TLSSkipVerify)
	data.DirectoriesOnly = gitopsDSBoolDefault(data.DirectoriesOnly)
	data.Force = gitopsDSBoolDefault(data.Force)

	payload := gitopsDSRepositoryPayload(data.Repository, data.Reference, data.TLSSkipVerify, data.Username, data.Password, data.SourceID)
	payload["dirOnly"] = data.DirectoriesOnly.ValueBool()
	if v, ok := gitopsDSString(data.Keyword); ok {
		payload["keyword"] = v
	}
	if v, ok := gitopsDSString(data.Include); ok {
		payload["include"] = v
	}

	searchURL := client.Endpoint + "/gitops/repo/files/search"
	if data.Force.ValueBool() {
		searchURL += "?force=true"
	}

	var paths []string
	if err := doJSON(ctx, client, http.MethodPost, searchURL, payload, &paths); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to search the repository for files: %w", err))
		return
	}

	data.Paths = gitopsDSStringList(gitopsDSSortedStrings(paths))
	data.ID = types.StringValue("portainer-gitops-repo-file-search-" + data.Repository.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
