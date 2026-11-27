package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*gitopsSourceConnectionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*gitopsSourceConnectionDataSource)(nil)
)

func newGitopsSourceConnectionDataSource() datasource.DataSource {
	return &gitopsSourceConnectionDataSource{}
}

type gitopsSourceConnectionDataSource struct{ client *APIClient }

type gitopsSourceConnectionModel struct {
	ID            types.String `tfsdk:"id"`
	SourceID      types.Int64  `tfsdk:"source_id"`
	URL           types.String `tfsdk:"url"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	TLSSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
	FailOnError   types.Bool   `tfsdk:"fail_on_error"`
	Success       types.Bool   `tfsdk:"success"`
	Error         types.String `tfsdk:"error"`
}

func (d *gitopsSourceConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_source_connection"
}

func (d *gitopsSourceConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"source_id": schema.Int64Attribute{
				Optional:    true,
				Validators:  []validator.Int64{int64validator.ConflictsWith(path.MatchRoot("url"))},
				Description: "Identifier of a stored GitOps source to test. Mutually exclusive with `url`; exactly one of the two is required.",
			},
			"url": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("source_id"))},
				Description: "Repository URL to test without storing a source, for validating credentials before creating one. Mutually exclusive with `source_id`.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username used for the test. Leave unset for an anonymous repository.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password or personal access token paired with `username`.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to skip TLS certificate verification during the test.",
			},
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether a failing connection makes the data source itself fail. Defaults to false, which reports the outcome in `success` and `error`.",
			},
			"success": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer could reach and authenticate against the repository.",
			},
			"error": schema.StringAttribute{
				Computed:    true,
				Description: "Reason the connection failed, empty on success.",
			},
		},
	}
}

func (d *gitopsSourceConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *gitopsSourceConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data gitopsSourceConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := d.client
	data.TLSSkipVerify = gitopsDSBoolDefault(data.TLSSkipVerify)
	data.FailOnError = gitopsDSBoolDefault(data.FailOnError)

	sourceID, hasID := gitopsDSInt(data.SourceID)
	repoURL, hasURL := gitopsDSString(data.URL)
	if hasID == hasURL {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("exactly one of source_id or url must be set"))
		return
	}

	payload := map[string]interface{}{"tlsSkipVerify": data.TLSSkipVerify.ValueBool()}
	if auth := gitopsDSAuth(data.Username.ValueString(), data.Password.ValueString()); auth != nil {
		payload["authentication"] = auth
	}

	// Portainer exposes two test endpoints: one for an ad-hoc repository and one
	// that re-tests a stored source, the latter taking the update payload so the
	// credentials of the stored source can be overridden for the test.
	var endpoint, id string
	if hasID {
		id = strconv.Itoa(sourceID)
		endpoint = fmt.Sprintf("%s/gitops/sources/%s/test", client.Endpoint, id)
	} else {
		payload["url"] = repoURL
		endpoint = client.Endpoint + "/gitops/sources/test"
		id = repoURL
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := doJSON(ctx, client, http.MethodPost, endpoint, payload, &result); err != nil {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("failed to test the GitOps source connection: %w", err))
		return
	}

	if !result.Success && data.FailOnError.ValueBool() {
		gitopsDSAddError(&resp.Diagnostics, fmt.Errorf("the GitOps source connection test failed: %s", result.Error))
		return
	}

	data.Success = types.BoolValue(result.Success)
	data.Error = types.StringValue(result.Error)
	data.ID = types.StringValue("gitops-source-test-" + id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
