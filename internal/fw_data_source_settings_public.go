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
	_ datasource.DataSource              = (*settingsPublicDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*settingsPublicDataSource)(nil)
)

func newSettingsPublicDataSource() datasource.DataSource { return &settingsPublicDataSource{} }

type settingsPublicDataSource struct{ client *APIClient }

type settingsPublicDataSourceModel struct {
	ID                        types.String `tfsdk:"id"`
	AuthenticationMethod      types.Int64  `tfsdk:"authentication_method"`
	EnableEdgeComputeFeatures types.Bool   `tfsdk:"enable_edge_compute_features"`
	RequiredPasswordLength    types.Int64  `tfsdk:"required_password_length"`
	RequiresSetupToken        types.Bool   `tfsdk:"requires_setup_token"`
	KubeconfigExpiry          types.String `tfsdk:"kubeconfig_expiry"`
	LogoURL                   types.String `tfsdk:"logo_url"`
	OAuthLoginURI             types.String `tfsdk:"oauth_login_uri"`
	OAuthLogoutURI            types.String `tfsdk:"oauth_logout_uri"`
	TeamSync                  types.Bool   `tfsdk:"team_sync"`
	IsDockerDesktopExtension  types.Bool   `tfsdk:"is_docker_desktop_extension"`
}

func (d *settingsPublicDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings_public"
}

func (d *settingsPublicDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}
	num := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Computed: true, Description: desc}
	}
	flag := func(desc string) schema.BoolAttribute { return schema.BoolAttribute{Computed: true, Description: desc} }
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                           schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"authentication_method":        num("Authentication method in use: 1 = internal, 2 = LDAP, 3 = OAuth."),
			"enable_edge_compute_features": flag("Whether Edge Compute features are enabled, which Edge environments and Edge stacks require."),
			"required_password_length":     num("Minimum password length Portainer enforces for internal accounts."),
			"requires_setup_token":         flag("Whether the instance still demands a setup token, meaning it has not been initialised."),
			"kubeconfig_expiry":            str("Lifetime of generated kubeconfig files as a duration string, `0` meaning they never expire."),
			"logo_url":                     str("Custom logo URL shown in the UI, empty when Portainer's own logo is used."),
			"oauth_login_uri":              str("URI users are sent to for OAuth login, empty when OAuth is not configured."),
			"oauth_logout_uri":             str("URI users are sent to on OAuth logout."),
			"team_sync":                    flag("Whether team membership is synchronised from the external authentication provider."),
			"is_docker_desktop_extension":  flag("Whether this Portainer runs as the Docker Desktop extension."),
		},
	}
}

func (d *settingsPublicDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *settingsPublicDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	// This endpoint needs no authentication, which is what makes it usable to
	// discover how to authenticate in the first place.
	var s struct {
		AuthenticationMethod      int    `json:"AuthenticationMethod"`
		EnableEdgeComputeFeatures bool   `json:"EnableEdgeComputeFeatures"`
		RequiredPasswordLength    int    `json:"RequiredPasswordLength"`
		RequiresSetupToken        bool   `json:"RequiresSetupToken"`
		KubeconfigExpiry          string `json:"KubeconfigExpiry"`
		LogoURL                   string `json:"LogoURL"`
		OAuthLoginURI             string `json:"OAuthLoginURI"`
		OAuthLogoutURI            string `json:"OAuthLogoutURI"`
		TeamSync                  bool   `json:"TeamSync"`
		IsDockerDesktopExtension  bool   `json:"IsDockerDesktopExtension"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/settings/public", nil, &s); err != nil {
		resp.Diagnostics.AddError("Failed to read the public Portainer settings", fmt.Sprintf("failed to read the public Portainer settings: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &settingsPublicDataSourceModel{
		ID:                        types.StringValue("portainer-public-settings"),
		AuthenticationMethod:      types.Int64Value(int64(s.AuthenticationMethod)),
		EnableEdgeComputeFeatures: types.BoolValue(s.EnableEdgeComputeFeatures),
		RequiredPasswordLength:    types.Int64Value(int64(s.RequiredPasswordLength)),
		RequiresSetupToken:        types.BoolValue(s.RequiresSetupToken),
		KubeconfigExpiry:          types.StringValue(s.KubeconfigExpiry),
		LogoURL:                   types.StringValue(s.LogoURL),
		OAuthLoginURI:             types.StringValue(s.OAuthLoginURI),
		OAuthLogoutURI:            types.StringValue(s.OAuthLogoutURI),
		TeamSync:                  types.BoolValue(s.TeamSync),
		IsDockerDesktopExtension:  types.BoolValue(s.IsDockerDesktopExtension),
	})...)
}
