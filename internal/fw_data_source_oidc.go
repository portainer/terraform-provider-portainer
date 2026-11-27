package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Data sources of Portainer's OIDC provider (Business Edition 3.1+):
// portainer_oidc_clients (GET /oidc_clients), portainer_oidc_client_sessions
// (GET /oidc_clients/{id}/sessions) and portainer_oidc_launcher
// (GET /oidc_launcher).

var (
	_ datasource.DataSourceWithConfigure = (*oidcClientsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*oidcClientSessionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*oidcLauncherDataSource)(nil)
)

// portainer_oidc_clients

func newOIDCClientsDataSource() datasource.DataSource { return &oidcClientsDataSource{} }

type oidcClientsDataSource struct{ client *APIClient }

type oidcClientsDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Clients types.List   `tfsdk:"clients"`
}

var oidcClientAttrTypes = map[string]attr.Type{
	"id":                        types.Int64Type,
	"client_id":                 types.StringType,
	"name":                      types.StringType,
	"description":               types.StringType,
	"homepage_url":              types.StringType,
	"permission":                types.StringType,
	"all_users":                 types.BoolType,
	"enabled":                   types.BoolType,
	"groups_claim":              types.BoolType,
	"show_in_launcher":          types.BoolType,
	"redirect_uris":             types.ListType{ElemType: types.StringType},
	"post_logout_redirect_uris": types.ListType{ElemType: types.StringType},
	"user_access_policies":      types.MapType{ElemType: types.Int64Type},
	"team_access_policies":      types.MapType{ElemType: types.Int64Type},
	"secrets":                   types.ListType{ElemType: types.ObjectType{AttrTypes: oidcSecretAttrTypes}},
}

func oidcClientValue(c oidcClientAPI) attr.Value {
	empty := types.MapValueMust(types.Int64Type, map[string]attr.Value{})
	return types.ObjectValueMust(oidcClientAttrTypes, map[string]attr.Value{
		"id":                        types.Int64Value(c.ID),
		"client_id":                 types.StringValue(c.ClientID),
		"name":                      types.StringValue(c.Name),
		"description":               types.StringValue(c.Description),
		"homepage_url":              types.StringValue(c.HomepageURL),
		"permission":                types.StringValue(c.Permission),
		"all_users":                 types.BoolValue(c.AllUsers),
		"enabled":                   types.BoolValue(c.Enabled),
		"groups_claim":              types.BoolValue(c.GroupsClaim),
		"show_in_launcher":          types.BoolValue(c.ShowInLauncher),
		"redirect_uris":             oidcStringList(c.RedirectURIs),
		"post_logout_redirect_uris": oidcStringList(c.PostLogoutRedirectURIs),
		"user_access_policies":      oidcPoliciesValue(c.UserAccessPolicies, empty),
		"team_access_policies":      oidcPoliciesValue(c.TeamAccessPolicies, empty),
		"secrets":                   oidcSecretsValue(c.Secrets),
	})
}

func (d *oidcClientsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_clients"
}

func (d *oidcClientsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the clients of Portainer's OIDC provider. Secret values are never returned. Business Edition 3.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Only list the clients with exactly this name.",
			},
			"clients": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: oidcClientAttrTypes},
				Description: "The matching clients, ordered by `id`. Each element has the attributes of `portainer_oidc_client` (without the secret values) plus `secrets`, the client's secrets without their values.",
			},
		},
	}
}

func (d *oidcClientsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *oidcClientsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oidcClientsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clients, err := oidcListClients(ctx, d.client, "portainer_oidc_clients")
	if err != nil {
		resp.Diagnostics.AddError("Failed to list OIDC clients", err.Error())
		return
	}
	name := data.Name.ValueString()
	out := make([]attr.Value, 0, len(clients))
	for _, c := range clients {
		if name == "" || c.Name == name {
			out = append(out, oidcClientValue(c))
		}
	}
	data.Clients = types.ListValueMust(types.ObjectType{AttrTypes: oidcClientAttrTypes}, out)
	data.ID = types.StringValue("portainer-oidc-clients-" + name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// portainer_oidc_client_sessions

func newOIDCClientSessionsDataSource() datasource.DataSource { return &oidcClientSessionsDataSource{} }

type oidcClientSessionsDataSource struct{ client *APIClient }

type oidcClientSessionsDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	ClientID types.Int64  `tfsdk:"client_id"`
	Sessions types.List   `tfsdk:"sessions"`
}

// oidcSessionAPI is oidcclients.sessionResponse.
type oidcSessionAPI struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	Username    string `json:"username"`
	CreatedAt   int64  `json:"createdAt"`
	ExpiresAt   int64  `json:"expiresAt"`
	LastUsedAt  int64  `json:"lastUsedAt"`
	Refreshable bool   `json:"refreshable"`
}

var oidcSessionAttrTypes = map[string]attr.Type{
	"id":           types.Int64Type,
	"user_id":      types.Int64Type,
	"username":     types.StringType,
	"created_at":   types.Int64Type,
	"expires_at":   types.Int64Type,
	"last_used_at": types.Int64Type,
	"refreshable":  types.BoolType,
}

func (d *oidcClientSessionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_client_sessions"
}

func (d *oidcClientSessionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the sessions of a client of Portainer's OIDC provider, one per sign-in. Business Edition 3.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"client_id": schema.Int64Attribute{
				Required:    true,
				Description: "Numeric Portainer identifier of the OIDC client (the `id` of `portainer_oidc_client`, not its OAuth `client_id`).",
			},
			"sessions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: oidcSessionAttrTypes},
				Description: "The client's sessions as Portainer returns them. Times are Unix times.",
			},
		},
	}
}

func (d *oidcClientSessionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *oidcClientSessionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oidcClientSessionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clientID := data.ClientID.ValueInt64()
	var sessions []oidcSessionAPI
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/oidc_clients/%d/sessions", d.client.Endpoint, clientID), nil, &sessions); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_oidc_client_sessions", requiresBE31OIDC)
		resp.Diagnostics.AddError("Failed to list OIDC client sessions", fmt.Sprintf("failed to list the sessions of OIDC client %d: %s", clientID, err))
		return
	}
	out := make([]attr.Value, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, types.ObjectValueMust(oidcSessionAttrTypes, map[string]attr.Value{
			"id":           types.Int64Value(s.ID),
			"user_id":      types.Int64Value(s.UserID),
			"username":     types.StringValue(s.Username),
			"created_at":   types.Int64Value(s.CreatedAt),
			"expires_at":   types.Int64Value(s.ExpiresAt),
			"last_used_at": types.Int64Value(s.LastUsedAt),
			"refreshable":  types.BoolValue(s.Refreshable),
		}))
	}
	data.Sessions = types.ListValueMust(types.ObjectType{AttrTypes: oidcSessionAttrTypes}, out)
	data.ID = types.StringValue(fmt.Sprintf("%d-sessions", clientID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// portainer_oidc_launcher

func newOIDCLauncherDataSource() datasource.DataSource { return &oidcLauncherDataSource{} }

type oidcLauncherDataSource struct{ client *APIClient }

type oidcLauncherDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Entries types.List   `tfsdk:"entries"`
}

var oidcLauncherAttrTypes = map[string]attr.Type{
	"name":         types.StringType,
	"description":  types.StringType,
	"homepage_url": types.StringType,
}

func (d *oidcLauncherDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_launcher"
}

func (d *oidcLauncherDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the entries of Portainer's launcher: the enabled OIDC clients shown in the launcher that the provider's user may sign in to. Business Edition 3.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"entries": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: oidcLauncherAttrTypes},
				Description: "The launcher entries, in Portainer's order.",
			},
		},
	}
}

func (d *oidcLauncherDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *oidcLauncherDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data oidcLauncherDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var entries []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		HomepageURL string `json:"homepageUrl"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/oidc_launcher", nil, &entries); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_oidc_launcher", requiresBE31OIDC)
		resp.Diagnostics.AddError("Failed to read the OIDC launcher", fmt.Sprintf("failed to read the OIDC launcher: %s", err))
		return
	}
	out := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		out = append(out, types.ObjectValueMust(oidcLauncherAttrTypes, map[string]attr.Value{
			"name":         types.StringValue(e.Name),
			"description":  types.StringValue(e.Description),
			"homepage_url": types.StringValue(e.HomepageURL),
		}))
	}
	data.Entries = types.ListValueMust(types.ObjectType{AttrTypes: oidcLauncherAttrTypes}, out)
	data.ID = types.StringValue("portainer-oidc-launcher")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
