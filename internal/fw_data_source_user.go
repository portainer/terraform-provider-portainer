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
	_ datasource.DataSource              = (*userDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userDataSource)(nil)
)

func newUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct{ client *APIClient }

type userDataSourceModel struct {
	ID                          types.String `tfsdk:"id"`
	Username                    types.String `tfsdk:"username"`
	Role                        types.Int64  `tfsdk:"role"`
	CreatedAuthenticationMethod types.String `tfsdk:"created_authentication_method"`
	LastAuthenticationMethod    types.String `tfsdk:"last_authentication_method"`
}

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the user."},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username of the Portainer user to look up. The data source will fail if no matching user is found.",
			},
			"role": schema.Int64Attribute{
				Computed:    true,
				Description: "Role assigned to the user in Portainer: 1 = admin, 2 = user.",
			},
			"created_authentication_method": schema.StringAttribute{
				Computed:    true,
				Description: "Authentication method used when the user was created (Portainer 3.0+). " + userAuthMethodDescription,
			},
			"last_authentication_method": schema.StringAttribute{
				Computed:    true,
				Description: "Authentication method used at the user's last login (Portainer 3.0+). " + userAuthMethodDescription,
			},
		},
	}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	username := data.Username.ValueString()

	userList, err := listUsers(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read user", fmt.Errorf("failed to list users: %w", err).Error())
		return
	}

	for _, u := range userList {
		if u.Username != username {
			continue
		}
		// The list leaves out the 3.0 authentication method fields; the
		// user's own endpoint has them.
		var detail portainerUser
		if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/users/%d", d.client.Endpoint, u.ID), nil, &detail); err == nil {
			u.CreatedAuthenticationMethod = detail.CreatedAuthenticationMethod
			u.LastAuthenticationMethod = detail.LastAuthenticationMethod
		}
		data.ID = types.StringValue(strconv.FormatInt(u.ID, 10))
		data.Role = types.Int64Value(u.Role)
		data.CreatedAuthenticationMethod = types.StringValue(authenticationMethodName(u.CreatedAuthenticationMethod))
		data.LastAuthenticationMethod = types.StringValue(authenticationMethodName(u.LastAuthenticationMethod))
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	resp.Diagnostics.AddError("Failed to read user", fmt.Sprintf("user %s not found", username))
}
