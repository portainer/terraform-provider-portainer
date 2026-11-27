package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsRepoRefsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsRepoRefsDataSource)(nil)
)

func newGitopsRepoRefsDataSource() datasource.DataSource { return &gitopsRepoRefsDataSource{} }

type gitopsRepoRefsDataSource struct{ client *APIClient }

type gitopsRepoRefsModel struct {
	ID              types.String `tfsdk:"id"`
	RepositoryURL   types.String `tfsdk:"repository_url"`
	Username        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	GitCredentialID types.Int64  `tfsdk:"git_credential_id"`
	TLSSkipVerify   types.Bool   `tfsdk:"tls_skip_verify"`
	Refs            types.List   `tfsdk:"refs"`
}

func (d *gitopsRepoRefsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_repo_refs"
}

func (d *gitopsRepoRefsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"repository_url": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Git repository",
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
			"refs": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Git references (branches and tags)",
			},
		},
	}
}

func (d *gitopsRepoRefsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsRepoRefsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsRepoRefsModel
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

	var refs []string
	if err := doJSON(ctx, client, "POST", client.Endpoint+"/gitops/repo/refs", payload, &refs); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to list Git repository refs: %w", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("gitops-repo-refs-%s", repoURL))
	data.Refs = gitopsDSStringList(refs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
