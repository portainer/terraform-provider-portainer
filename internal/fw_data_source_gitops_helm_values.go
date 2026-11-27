package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsHelmValuesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsHelmValuesDataSource)(nil)
)

func newGitopsHelmValuesDataSource() datasource.DataSource { return &gitopsHelmValuesDataSource{} }

type gitopsHelmValuesDataSource struct{ client *APIClient }

type gitopsHelmValuesModel struct {
	ID             types.String `tfsdk:"id"`
	Repository     types.String `tfsdk:"repository"`
	Reference      types.String `tfsdk:"reference"`
	ValuesFiles    types.List   `tfsdk:"values_files"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	SourceID       types.Int64  `tfsdk:"source_id"`
	TLSSkipVerify  types.Bool   `tfsdk:"tls_skip_verify"`
	MergedValues   types.String `tfsdk:"merged_values"`
	FilesProcessed types.List   `tfsdk:"files_processed"`
	CommitHash     types.String `tfsdk:"commit_hash"`
}

func (d *gitopsHelmValuesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_helm_values"
}

func (d *gitopsHelmValuesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"repository": schema.StringAttribute{
				Required:    true,
				Description: "URL of the git repository holding the values files.",
			},
			"reference": schema.StringAttribute{
				Required:    true,
				Description: "Git reference to read at, for example `refs/heads/main`.",
			},
			"values_files": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Paths of the values files to merge, in order. Later files win, the same way `helm -f` treats them.",
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
			"merged_values": schema.StringAttribute{
				Computed:    true,
				Description: "The merged values as YAML, which is what a Helm deployment from these files would receive.",
			},
			"files_processed": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The files that were actually merged, in the order they were applied. A file missing from this list was not found in the repository.",
			},
			"commit_hash": schema.StringAttribute{
				Computed:    true,
				Description: "Commit the values were read at. Watch this to notice the repository moving under a configuration.",
			},
		},
	}
}

func (d *gitopsHelmValuesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsHelmValuesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsHelmValuesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	data.TLSSkipVerify = gitopsDSBoolDefault(data.TLSSkipVerify)

	payload := gitopsDSRepositoryPayload(data.Repository, data.Reference, data.TLSSkipVerify, data.Username, data.Password, data.SourceID)
	files := []string{}
	resp.Diagnostics.Append(data.ValuesFiles.ElementsAs(ctx, &files, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload["valuesFiles"] = files

	var response struct {
		MergedValues   string          `json:"mergedValues"`
		FilesProcessed json.RawMessage `json:"filesProcessed"`
		CommitHash     string          `json:"commitHash"`
	}
	valuesURL := client.Endpoint + "/gitops/repo/helm/values"
	if err := doJSON(ctx, client, http.MethodPost, valuesURL, payload, &response); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to preview the merged Helm values: %w", err))
		return
	}

	// The processed list is kept in the order Portainer applied the files,
	// because that order is what decides which value wins.
	var processed []string
	if len(response.FilesProcessed) > 0 {
		if err := json.Unmarshal(response.FilesProcessed, &processed); err != nil {
			gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to parse the list of processed values files: %w", err))
			return
		}
	}

	data.MergedValues = types.StringValue(response.MergedValues)
	data.FilesProcessed = gitopsDSStringList(processed)
	data.CommitHash = types.StringValue(response.CommitHash)
	data.ID = types.StringValue("portainer-gitops-helm-values-" + data.Repository.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
