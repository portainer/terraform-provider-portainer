package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*sharedGitCredentialDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sharedGitCredentialDataSource)(nil)
)

func newSharedGitCredentialDataSource() datasource.DataSource {
	return &sharedGitCredentialDataSource{}
}

type sharedGitCredentialDataSource struct{ client *APIClient }

type sharedGitCredentialDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Username          types.String `tfsdk:"username"`
	AuthorizationType types.Int64  `tfsdk:"authorization_type"`
	UserID            types.Int64  `tfsdk:"user_id"`
}

func (d *sharedGitCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_git_credential"
}

func (d *sharedGitCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the shared git credential.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the shared git credential to look up",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username for git authentication",
			},
			"authorization_type": schema.Int64Attribute{
				Computed:    true,
				Description: "Authorization type: 0 = Basic, 1 = Token",
			},
			"user_id": schema.Int64Attribute{
				Computed:    true,
				Description: "User ID of the credential owner",
			},
		},
	}
}

func (d *sharedGitCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *sharedGitCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config sharedGitCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var credentials []gitCredentialAPI
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/cloud/gitcredentials", nil, &credentials); err != nil {
		resp.Diagnostics.AddError("Failed to list shared git credentials", fmt.Errorf("failed to list shared git credentials: %w", err).Error())
		return
	}

	name := config.Name.ValueString()
	for _, c := range credentials {
		if c.Name == name {
			config.ID = types.StringValue(strconv.Itoa(c.ID))
			config.Username = types.StringValue(c.Username)
			config.AuthorizationType = types.Int64Value(int64(c.AuthorizationType))
			config.UserID = types.Int64Value(int64(c.UserID))
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
	}

	resp.Diagnostics.AddError("Shared git credential not found", fmt.Sprintf("shared git credential with name %q not found", name))
}
