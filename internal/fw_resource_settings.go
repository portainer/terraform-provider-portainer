package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type SettingsPayload struct {
	EdgePortainerURL          string `json:"EdgePortainerURL,omitempty"`
	AuthenticationMethod      int    `json:"authenticationMethod,omitempty"`
	LogoURL                   string `json:"logoURL,omitempty"`
	SnapshotInterval          string `json:"snapshotInterval,omitempty"`
	TemplatesURL              string `json:"templatesURL,omitempty"`
	EnableEdgeComputeFeatures bool   `json:"enableEdgeComputeFeatures,omitempty"`
	EnforceEdgeID             bool   `json:"enforceEdgeID,omitempty"`
	UserSessionTimeout        string `json:"userSessionTimeout,omitempty"`
	KubeconfigExpiry          string `json:"kubeconfigExpiry,omitempty"`
	KubectlShellImage         string `json:"kubectlShellImage,omitempty"`
	HelmRepositoryURL         string `json:"helmRepositoryURL,omitempty"`
	// AddonsCatalogURL is Business Edition only; an empty value is omitted so it
	// never reaches a CE instance and never clears a configured catalog.
	AddonsCatalogURL            string                `json:"AddonsCatalogURL,omitempty"`
	TrustOnFirstConnect         bool                  `json:"trustOnFirstConnect,omitempty"`
	EdgeAgentCheckinInterval    int                   `json:"edgeAgentCheckinInterval,omitempty"`
	BlackListedLabels           []LabelPair           `json:"blackListedLabels,omitempty"`
	GlobalDeploymentOptions     *GlobalDeploymentOpts `json:"globalDeploymentOptions,omitempty"`
	InternalAuthSettings        *InternalAuthSettings `json:"internalAuthSettings,omitempty"`
	OAuthSettings               *OAuthSettings        `json:"oauthSettings,omitempty"`
	LDAPSettings                *LDAPSettings         `json:"ldapsettings,omitempty"`
	DisableKubeRolesSync        bool                  `json:"DisableKubeRolesSync,omitempty"`
	DisableKubeShell            bool                  `json:"DisableKubeShell,omitempty"`
	DisableKubeconfigDownload   bool                  `json:"DisableKubeconfigDownload,omitempty"`
	DisplayDonationHeader       bool                  `json:"DisplayDonationHeader,omitempty"`
	DisplayExternalContributors bool                  `json:"DisplayExternalContributors,omitempty"`
	IsDockerDesktopExtension    bool                  `json:"IsDockerDesktopExtension,omitempty"`
	ForceSecureCookies          bool                  `json:"ForceSecureCookies,omitempty"`
}

// settingsUpdateRequest is the PUT /settings body. The boolean switches of
// SettingsPayload are omitempty, so turning one off was never sent and the
// setting stayed on; these pointers shadow them (the outer field wins in
// encoding/json) and carry false whenever it is configured.
type settingsUpdateRequest struct {
	SettingsPayload
	EnableEdgeComputeFeatures *bool `json:"enableEdgeComputeFeatures,omitempty"`
	EnforceEdgeID             *bool `json:"enforceEdgeID,omitempty"`
	TrustOnFirstConnect       *bool `json:"trustOnFirstConnect,omitempty"`
	DisableKubeRolesSync      *bool `json:"DisableKubeRolesSync,omitempty"`
	DisableKubeShell          *bool `json:"DisableKubeShell,omitempty"`
	DisableKubeconfigDownload *bool `json:"DisableKubeconfigDownload,omitempty"`
	ForceSecureCookies        *bool `json:"ForceSecureCookies,omitempty"`
	// The Business Edition settings below are only sent when configured, so
	// a configuration without them sends what it always did.
	CustomLoginBanner *string                   `json:"CustomLoginBanner,omitempty"`
	AutoPatchSettings *settingsAutoPatchPayload `json:"AutoPatchSettings,omitempty"`
	Edge              *settingsEdgePayload      `json:"Edge,omitempty"`
	// The OIDC provider settings (Business Edition 3.1+) are top-level
	// fields of PUT /settings; GET returns them in OIDCProviderSettings.
	OIDCProviderPublicURL                    *string `json:"OIDCProviderPublicURL,omitempty"`
	OIDCProviderAccessTokenLifetime          *string `json:"OIDCProviderAccessTokenLifetime,omitempty"`
	OIDCProviderRefreshTokenIdleLifetime     *string `json:"OIDCProviderRefreshTokenIdleLifetime,omitempty"`
	OIDCProviderRefreshTokenAbsoluteLifetime *string `json:"OIDCProviderRefreshTokenAbsoluteLifetime,omitempty"`
}

// settingsAutoPatchPayload is settings.autoPatchSettingsPayload. Only the
// known values of the block are sent; Portainer keeps the others.
type settingsAutoPatchPayload struct {
	Enabled             *bool   `json:"Enabled,omitempty"`
	PatchCron           *string `json:"PatchCron,omitempty"`
	PortainerRepository *string `json:"PortainerRepository,omitempty"`
	RegistryID          *int64  `json:"RegistryID,omitempty"`
	UpdaterRepository   *string `json:"UpdaterRepository,omitempty"`
}

// settingsEdgePayload is the Edge object of settings.settingsUpdatePayload.
type settingsEdgePayload struct {
	AsyncMode           *bool                `json:"AsyncMode,omitempty"`
	CommandInterval     *int64               `json:"CommandInterval,omitempty"`
	PingInterval        *int64               `json:"PingInterval,omitempty"`
	SnapshotInterval    *int64               `json:"SnapshotInterval,omitempty"`
	TunnelServerAddress *string              `json:"TunnelServerAddress,omitempty"`
	MTLS                *settingsMTLSPayload `json:"MTLS,omitempty"`
}

// settingsMTLSPayload is settings.mTLSPayload: the certificates themselves,
// which Portainer stores as files and never returns.
type settingsMTLSPayload struct {
	UseSeparateCert *bool   `json:"UseSeparateCert,omitempty"`
	CaCert          *string `json:"CaCert,omitempty"`
	Cert            *string `json:"Cert,omitempty"`
	Key             *string `json:"Key,omitempty"`
}

// settingsResponse is GET /settings: SettingsPayload plus the Business
// Edition settings read back only by this resource. They are not part of
// SettingsPayload so that nothing else ever sends them back.
type settingsResponse struct {
	SettingsPayload
	CustomLoginBanner string `json:"CustomLoginBanner"`
	AutoPatchSettings *struct {
		Enabled             bool   `json:"Enabled"`
		PatchCron           string `json:"PatchCron"`
		PortainerRepository string `json:"PortainerRepository"`
		RegistryID          int64  `json:"RegistryID"`
		UpdaterRepository   string `json:"UpdaterRepository"`
	} `json:"AutoPatchSettings"`
	Edge *struct {
		AsyncMode           bool   `json:"AsyncMode"`
		CommandInterval     int64  `json:"CommandInterval"`
		PingInterval        int64  `json:"PingInterval"`
		SnapshotInterval    int64  `json:"SnapshotInterval"`
		TunnelServerAddress string `json:"TunnelServerAddress"`
		MTLS                *struct {
			UseSeparateCert bool `json:"UseSeparateCert"`
		} `json:"MTLS"`
	} `json:"Edge"`
	OIDCProviderSettings *struct {
		PublicURL                    string `json:"PublicURL"`
		AccessTokenLifetime          string `json:"AccessTokenLifetime"`
		RefreshTokenIdleLifetime     string `json:"RefreshTokenIdleLifetime"`
		RefreshTokenAbsoluteLifetime string `json:"RefreshTokenAbsoluteLifetime"`
	} `json:"OIDCProviderSettings"`
}

type LabelPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type GlobalDeploymentOpts struct {
	HideStacksFunctionality bool `json:"hideStacksFunctionality,omitempty"`
}

type InternalAuthSettings struct {
	RequiredPasswordLength int `json:"requiredPasswordLength,omitempty"`
}

// --- OAuth structs ---

type OAuthClaimMapping struct {
	ClaimValRegex string `json:"ClaimValRegex,omitempty"`
	Team          int    `json:"Team,omitempty"`
}

type OAuthTeamMemberships struct {
	AdminAutoPopulate         bool                `json:"AdminAutoPopulate,omitempty"`
	AdminGroupClaimsRegexList []string            `json:"AdminGroupClaimsRegexList,omitempty"`
	OAuthClaimMappings        []OAuthClaimMapping `json:"OAuthClaimMappings,omitempty"`
	OAuthClaimName            string              `json:"OAuthClaimName,omitempty"`
}

type OAuthSettings struct {
	AccessTokenURI              string                `json:"AccessTokenURI,omitempty"`
	AuthStyle                   int                   `json:"AuthStyle,omitempty"`
	AuthorizationURI            string                `json:"AuthorizationURI,omitempty"`
	ClientID                    string                `json:"ClientID,omitempty"`
	ClientSecret                string                `json:"ClientSecret,omitempty"`
	DefaultTeamID               int                   `json:"DefaultTeamID,omitempty"`
	LogoutURI                   string                `json:"LogoutURI,omitempty"`
	OAuthAutoCreateUsers        bool                  `json:"OAuthAutoCreateUsers,omitempty"`
	RedirectURI                 string                `json:"RedirectURI,omitempty"`
	ResourceURI                 string                `json:"ResourceURI,omitempty"`
	SSO                         bool                  `json:"SSO,omitempty"`
	Scopes                      string                `json:"Scopes,omitempty"`
	UserIdentifier              string                `json:"UserIdentifier,omitempty"`
	KubeSecretKey               []int                 `json:"KubeSecretKey,omitempty"`
	HideInternalAuth            bool                  `json:"HideInternalAuth,omitempty"`
	MicrosoftTenantID           string                `json:"MicrosoftTenantID,omitempty"`
	OAuthAutoMapTeamMemberships bool                  `json:"OAuthAutoMapTeamMemberships,omitempty"`
	TeamMemberships             *OAuthTeamMemberships `json:"TeamMemberships,omitempty"`
}

// --- LDAP structs ---

type LDAPSettings struct {
	AnonymousMode       bool            `json:"AnonymousMode,omitempty"`
	AutoCreateUsers     bool            `json:"AutoCreateUsers,omitempty"`
	Password            string          `json:"Password,omitempty"`
	ReaderDN            string          `json:"ReaderDN,omitempty"`
	StartTLS            bool            `json:"StartTLS,omitempty"`
	URL                 string          `json:"URL,omitempty"`
	SearchSettings      []SearchSetting `json:"SearchSettings,omitempty"`
	GroupSearchSettings []GroupSearch   `json:"GroupSearchSettings,omitempty"`
	TLSConfig           *TLSConfig      `json:"TLSConfig,omitempty"`
}

type SearchSetting struct {
	BaseDN            string `json:"BaseDN,omitempty"`
	Filter            string `json:"Filter,omitempty"`
	UserNameAttribute string `json:"UserNameAttribute,omitempty"`
}

type GroupSearch struct {
	GroupAttribute string `json:"GroupAttribute,omitempty"`
	GroupBaseDN    string `json:"GroupBaseDN,omitempty"`
	GroupFilter    string `json:"GroupFilter,omitempty"`
}

type TLSConfig struct {
	TLS           bool   `json:"TLS,omitempty"`
	TLSCACert     string `json:"TLSCACert,omitempty"`
	TLSCert       string `json:"TLSCert,omitempty"`
	TLSKey        string `json:"TLSKey,omitempty"`
	TLSSkipVerify bool   `json:"TLSSkipVerify,omitempty"`
}

// ---- framework resource ----

var (
	_ resource.Resource                   = (*settingsResource)(nil)
	_ resource.ResourceWithConfigure      = (*settingsResource)(nil)
	_ resource.ResourceWithImportState    = (*settingsResource)(nil)
	_ resource.ResourceWithValidateConfig = (*settingsResource)(nil)
)

const settingsID = "portainer-settings"

func newSettingsResource() resource.Resource { return &settingsResource{} }

type settingsResource struct{ client *APIClient }

type settingsModel struct {
	ID                          types.String                `tfsdk:"id"`
	EdgePortainerURL            types.String                `tfsdk:"edge_portainer_url"`
	AuthenticationMethod        types.Int64                 `tfsdk:"authentication_method"`
	LogoURL                     types.String                `tfsdk:"logo_url"`
	SnapshotInterval            types.String                `tfsdk:"snapshot_interval"`
	TemplatesURL                types.String                `tfsdk:"templates_url"`
	EnableEdgeComputeFeatures   types.Bool                  `tfsdk:"enable_edge_compute_features"`
	EnforceEdgeID               types.Bool                  `tfsdk:"enforce_edge_id"`
	UserSessionTimeout          types.String                `tfsdk:"user_session_timeout"`
	KubeconfigExpiry            types.String                `tfsdk:"kubeconfig_expiry"`
	KubectlShellImage           types.String                `tfsdk:"kubectl_shell_image"`
	AddonsCatalogURL            types.String                `tfsdk:"addons_catalog_url"`
	HelmRepositoryURL           types.String                `tfsdk:"helm_repository_url"`
	DisableKubeRolesSync        types.Bool                  `tfsdk:"disable_kube_roles_sync"`
	DisableKubeShell            types.Bool                  `tfsdk:"disable_kube_shell"`
	DisableKubeconfigDownload   types.Bool                  `tfsdk:"disable_kubeconfig_download"`
	DisplayDonationHeader       types.Bool                  `tfsdk:"display_donation_header"`
	DisplayExternalContributors types.Bool                  `tfsdk:"display_external_contributors"`
	IsDockerDesktopExtension    types.Bool                  `tfsdk:"is_docker_desktop_extension"`
	OAuthClientSecretWO         types.String                `tfsdk:"oauth_client_secret_wo"`
	OAuthClientSecretWOVersion  types.Int64                 `tfsdk:"oauth_client_secret_wo_version"`
	LDAPPasswordWO              types.String                `tfsdk:"ldap_password_wo"`
	LDAPPasswordWOVersion       types.Int64                 `tfsdk:"ldap_password_wo_version"`
	ForceSecureCookies          types.Bool                  `tfsdk:"force_secure_cookies"`
	TrustOnFirstConnect         types.Bool                  `tfsdk:"trust_on_first_connect"`
	EdgeAgentCheckinInterval    types.Int64                 `tfsdk:"edge_agent_checkin_interval"`
	BlackListedLabels           []settingsLabelModel        `tfsdk:"black_listed_labels"`
	GlobalDeploymentOptions     []settingsGlobalOptsModel   `tfsdk:"global_deployment_options"`
	InternalAuthSettings        []settingsInternalAuthModel `tfsdk:"internal_auth_settings"`
	OAuthSettings               []settingsOAuthModel        `tfsdk:"oauth_settings"`
	LDAPSettings                []settingsLDAPSettingsModel `tfsdk:"ldap_settings"`
	CustomLoginBanner           types.String                `tfsdk:"custom_login_banner"`
	AutoPatchSettings           []settingsAutoPatchModel    `tfsdk:"auto_patch_settings"`
	Edge                        []settingsEdgeModel         `tfsdk:"edge"`
	OIDCProvider                []settingsOIDCProviderModel `tfsdk:"oidc_provider"`
}

type settingsOIDCProviderModel struct {
	PublicURL                    types.String `tfsdk:"public_url"`
	AccessTokenLifetime          types.String `tfsdk:"access_token_lifetime"`
	RefreshTokenIdleLifetime     types.String `tfsdk:"refresh_token_idle_lifetime"`
	RefreshTokenAbsoluteLifetime types.String `tfsdk:"refresh_token_absolute_lifetime"`
}

type settingsAutoPatchModel struct {
	Enabled             types.Bool   `tfsdk:"enabled"`
	PatchCron           types.String `tfsdk:"patch_cron"`
	PortainerRepository types.String `tfsdk:"portainer_repository"`
	RegistryID          types.Int64  `tfsdk:"registry_id"`
	UpdaterRepository   types.String `tfsdk:"updater_repository"`
}

type settingsEdgeModel struct {
	AsyncMode           types.Bool          `tfsdk:"async_mode"`
	CommandInterval     types.Int64         `tfsdk:"command_interval"`
	PingInterval        types.Int64         `tfsdk:"ping_interval"`
	SnapshotInterval    types.Int64         `tfsdk:"snapshot_interval"`
	TunnelServerAddress types.String        `tfsdk:"tunnel_server_address"`
	MTLS                []settingsMTLSModel `tfsdk:"mtls"`
}

type settingsMTLSModel struct {
	UseSeparateCert types.Bool   `tfsdk:"use_separate_cert"`
	CaCert          types.String `tfsdk:"ca_cert"`
	Cert            types.String `tfsdk:"cert"`
	Key             types.String `tfsdk:"key"`
}

type settingsLabelModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type settingsGlobalOptsModel struct {
	HideStacksFunctionality types.Bool `tfsdk:"hide_stacks_functionality"`
}

type settingsInternalAuthModel struct {
	RequiredPasswordLength types.Int64 `tfsdk:"required_password_length"`
}

type settingsOAuthModel struct {
	AccessTokenURI              types.String                  `tfsdk:"access_token_uri"`
	AuthStyle                   types.Int64                   `tfsdk:"auth_style"`
	AuthorizationURI            types.String                  `tfsdk:"authorization_uri"`
	ClientID                    types.String                  `tfsdk:"client_id"`
	ClientSecret                types.String                  `tfsdk:"client_secret"`
	DefaultTeamID               types.Int64                   `tfsdk:"default_team_id"`
	LogoutURI                   types.String                  `tfsdk:"logout_uri"`
	OAuthAutoCreateUsers        types.Bool                    `tfsdk:"oauth_auto_create_users"`
	RedirectURI                 types.String                  `tfsdk:"redirect_uri"`
	ResourceURI                 types.String                  `tfsdk:"resource_uri"`
	SSO                         types.Bool                    `tfsdk:"sso"`
	Scopes                      types.String                  `tfsdk:"scopes"`
	UserIdentifier              types.String                  `tfsdk:"user_identifier"`
	KubeSecretKey               types.List                    `tfsdk:"kube_secret_key"`
	HideInternalAuth            types.Bool                    `tfsdk:"hide_internal_auth"`
	MicrosoftTenantID           types.String                  `tfsdk:"microsoft_tenant_id"`
	OAuthAutoMapTeamMemberships types.Bool                    `tfsdk:"oauth_auto_map_team_memberships"`
	TeamMemberships             []settingsTeamMembershipModel `tfsdk:"team_memberships"`
}

type settingsTeamMembershipModel struct {
	OAuthClaimName            types.String                `tfsdk:"oauth_claim_name"`
	AdminAutoPopulate         types.Bool                  `tfsdk:"admin_auto_populate"`
	AdminGroupClaimsRegexList types.List                  `tfsdk:"admin_group_claims_regex_list"`
	OAuthClaimMappings        []settingsClaimMappingModel `tfsdk:"oauth_claim_mappings"`
}

type settingsClaimMappingModel struct {
	ClaimValRegex types.String `tfsdk:"claim_val_regex"`
	Team          types.Int64  `tfsdk:"team"`
}

type settingsLDAPSettingsModel struct {
	AnonymousMode       types.Bool                `tfsdk:"anonymous_mode"`
	AutoCreateUsers     types.Bool                `tfsdk:"auto_create_users"`
	Password            types.String              `tfsdk:"password"`
	ReaderDN            types.String              `tfsdk:"reader_dn"`
	StartTLS            types.Bool                `tfsdk:"start_tls"`
	URL                 types.String              `tfsdk:"url"`
	SearchSettings      []settingsLDAPSearchModel `tfsdk:"search_settings"`
	GroupSearchSettings []settingsLDAPGroupModel  `tfsdk:"group_search_settings"`
	TLSConfig           []settingsLDAPTLSModel    `tfsdk:"tls_config"`
}

type settingsLDAPSearchModel struct {
	BaseDN            types.String `tfsdk:"base_dn"`
	Filter            types.String `tfsdk:"filter"`
	UserNameAttribute types.String `tfsdk:"user_name_attribute"`
}

type settingsLDAPGroupModel struct {
	GroupAttribute types.String `tfsdk:"group_attribute"`
	GroupBaseDN    types.String `tfsdk:"group_base_dn"`
	GroupFilter    types.String `tfsdk:"group_filter"`
}

type settingsLDAPTLSModel struct {
	TLS           types.Bool   `tfsdk:"tls"`
	TLSCACert     types.String `tfsdk:"tls_ca_cert"`
	TLSCert       types.String `tfsdk:"tls_cert"`
	TLSKey        types.String `tfsdk:"tls_key"`
	TLSSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
}

// Optional+computed attributes keep their state value when the
// configuration leaves them out, as SDKv2 did.
func ocString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
}

// settingsOIDCDuration is an optional, computed Go duration of the
// oidc_provider block.
func settingsOIDCDuration(desc string) schema.StringAttribute {
	a := ocString(desc)
	a.Validators = []validator.String{durationValidator{}}
	return a
}

func ocBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
}

func ocInt(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
}

func ocList(elem attr.Type, desc string) schema.ListAttribute {
	return schema.ListAttribute{Optional: true, Computed: true, ElementType: elem, Description: desc, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}
}

func computedBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{Computed: true, Description: desc, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
}

func maxOne() []validator.List { return []validator.List{listvalidator.SizeAtMost(1)} }

func (r *settingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings"
}

func (r *settingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	clientSecret := ocString("OAuth client secret. Stored in state as a sensitive value; use the top-level `oauth_client_secret_wo` to keep it out.")
	clientSecret.Sensitive = true
	kubeSecretKey := ocList(types.Int64Type, "Byte-array key used to encrypt OAuth-derived Kubernetes secrets. Stored in state.")
	kubeSecretKey.Sensitive = true
	ldapPassword := ocString("Password used by the LDAP reader account. Stored in state as a sensitive value; use the top-level `ldap_password_wo` to keep it out.")
	ldapPassword.Sensitive = true
	tlsKey := ocString("PEM-encoded private key matching the client certificate.")
	tlsKey.Sensitive = true
	mtlsKey := schema.StringAttribute{Optional: true, Sensitive: true, Description: "PEM-encoded private key of the mTLS certificate. Portainer never returns it."}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"edge_portainer_url":            ocString("Public URL of the Portainer instance that edge agents use to reach back for polling and tunneling."),
			"authentication_method":         ocInt("Authentication method used by Portainer. 1 = internal, 2 = LDAP, 3 = OAuth."),
			"logo_url":                      ocString("URL of a custom logo displayed in the Portainer UI."),
			"snapshot_interval":             ocString("Interval at which Portainer snapshots endpoints (e.g. \"5m\", \"1h\")."),
			"templates_url":                 ocString("URL of the JSON file with application templates used by the Portainer UI."),
			"enable_edge_compute_features":  ocBool("Whether Edge Compute features (edge agents, edge stacks, edge jobs) are enabled."),
			"enforce_edge_id":               ocBool("Whether Portainer enforces a matching edge identifier when an edge agent connects."),
			"user_session_timeout":          ocString("Duration after which an idle user session expires (e.g. \"8h\")."),
			"kubeconfig_expiry":             ocString("Validity period of kubeconfig files generated for users (e.g. \"24h\", \"0\" for never)."),
			"kubectl_shell_image":           ocString("Container image used for the in-browser kubectl shell."),
			"addons_catalog_url":            ocString("URL the add-on catalog is fetched from. Business Edition only. Leave unset to keep the built-in catalog."),
			"helm_repository_url":           ocString("Global Helm repository URL used by Portainer for chart browsing."),
			"disable_kube_roles_sync":       ocBool("Whether automatic synchronization of Kubernetes roles is disabled. Business Edition only; Community Edition ignores it."),
			"disable_kube_shell":            ocBool("Whether the in-browser Kubernetes shell is disabled for users. Business Edition only; Community Edition ignores it."),
			"disable_kubeconfig_download":   ocBool("Whether downloading kubeconfig files from the Portainer UI is disabled. Business Edition only; Community Edition ignores it."),
			"display_donation_header":       computedBool("Whether the donation header banner is shown in the Portainer UI. Read-only: Portainer does not accept it through the settings API."),
			"display_external_contributors": computedBool("Whether external contributors are displayed in the Portainer UI. Read-only: Portainer does not accept it through the settings API."),
			"is_docker_desktop_extension":   computedBool("Whether this Portainer instance runs as a Docker Desktop extension. Read-only: Portainer detects it at startup."),
			"oauth_client_secret_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("oauth_settings").AtAnyListIndex().AtName("client_secret")),
					stringvalidator.AlsoRequires(path.MatchRoot("oauth_client_secret_wo_version")),
				},
				Description: "Write-only OAuth client secret used instead of `oauth_settings.client_secret` (supports ephemeral values; never stored in Terraform state). Requires Terraform 1.11 or later.",
			},
			"oauth_client_secret_wo_version": gsbWriteOnlyVersionAttribute("oauth_client_secret_wo", false),
			"ldap_password_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("ldap_settings").AtAnyListIndex().AtName("password")),
					stringvalidator.AlsoRequires(path.MatchRoot("ldap_password_wo_version")),
				},
				Description: "Write-only password of the LDAP reader account used instead of `ldap_settings.password` (supports ephemeral values; never stored in Terraform state). Requires Terraform 1.11 or later.",
			},
			"ldap_password_wo_version":    gsbWriteOnlyVersionAttribute("ldap_password_wo", false),
			"force_secure_cookies":        ocBool("Whether Portainer always sets the Secure attribute on its authentication cookies, whatever scheme it detects. Enable it when Portainer runs behind a TLS-terminating proxy. Portainer 2.45 and later."),
			"trust_on_first_connect":      ocBool("Whether edge agents are automatically trusted on their first connection."),
			"edge_agent_checkin_interval": ocInt("Heartbeat interval (in seconds) used by edge agents to poll Portainer."),
			"custom_login_banner":         ocString("Plain text shown on the login page, such as a notice or an agreement; an empty string hides it. Business Edition only. Sent only when set."),
		},
		// SDKv2 declared these blocks optional and computed. A framework block
		// cannot be computed: Terraform requires as many blocks in the plan as
		// in the configuration. Read therefore only reads back the blocks the
		// state already has (see settingsReadMode).
		Blocks: map[string]schema.Block{
			"black_listed_labels": schema.ListNestedBlock{
				Description: "List of container labels that Portainer should hide from the UI.",
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"name":  schema.StringAttribute{Required: true, Description: "Name of the container label to hide."},
					"value": schema.StringAttribute{Required: true, Description: "Value of the container label to match for hiding."},
				}},
			},
			"global_deployment_options": schema.ListNestedBlock{
				Description: "Global deployment options applied across environments.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"hide_stacks_functionality": ocBool("Whether the stacks functionality is hidden from non-admin users."),
				}},
			},
			"internal_auth_settings": schema.ListNestedBlock{
				Description: "Settings for Portainer's internal authentication provider, such as the password policy.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"required_password_length": ocInt("Minimum password length required for internally-managed users."),
				}},
			},
			"oauth_settings": schema.ListNestedBlock{
				Description: "Configuration for the OAuth authentication provider.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"access_token_uri":                ocString("Token endpoint URL used to exchange the authorization code for an access token."),
						"auth_style":                      ocInt("OAuth client authentication style passed to the token endpoint (0 = auto-detect, 1 = params, 2 = HTTP Basic header)."),
						"authorization_uri":               ocString("Authorization endpoint URL of the OAuth provider."),
						"client_id":                       ocString("OAuth client identifier registered with the provider."),
						"client_secret":                   clientSecret,
						"default_team_id":                 ocInt("Identifier of the team that new OAuth users are added to by default. 0 means no default team."),
						"logout_uri":                      ocString("URL the user is redirected to after logging out from Portainer."),
						"oauth_auto_create_users":         ocBool("Whether Portainer automatically creates a local user record on the first OAuth login."),
						"redirect_uri":                    ocString("Redirect URI registered with the OAuth provider; must match Portainer's callback URL."),
						"resource_uri":                    ocString("Resource endpoint URL used to fetch the authenticated user's profile."),
						"sso":                             ocBool("Whether single sign-on is enabled, redirecting users straight to the OAuth provider."),
						"scopes":                          ocString("OAuth scopes requested from the provider (space- or comma-separated)."),
						"user_identifier":                 ocString("Claim used as the unique user identifier from the OAuth provider response."),
						"kube_secret_key":                 kubeSecretKey,
						"hide_internal_auth":              ocBool("Whether to hide the internal username/password login form when OAuth is configured."),
						"microsoft_tenant_id":             ocString("Microsoft Entra/Azure AD tenant identifier used when the OAuth provider is Microsoft."),
						"oauth_auto_map_team_memberships": ocBool("Whether Portainer automatically maps Portainer team memberships from OAuth claims."),
					},
					Blocks: map[string]schema.Block{
						"team_memberships": schema.ListNestedBlock{
							Description: "Rules that map OAuth claims to Portainer team memberships and admin role.",
							Validators:  maxOne(),
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"oauth_claim_name":              ocString("Name of the OAuth claim that carries the user's group/team membership values."),
									"admin_auto_populate":           ocBool("Whether Portainer auto-grants admin rights to users whose claim values match the admin regex list."),
									"admin_group_claims_regex_list": ocList(types.StringType, "List of regular expressions that, when matching a claim value, mark the user as Portainer admin."),
								},
								Blocks: map[string]schema.Block{
									"oauth_claim_mappings": schema.ListNestedBlock{
										Description: "List of mappings from OAuth claim value regexes to Portainer team IDs.",
										NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
											"claim_val_regex": ocString("Regular expression matched against the OAuth claim value to trigger this mapping."),
											"team":            ocInt("Identifier of the Portainer team that matching users are added to."),
										}},
									},
								},
							},
						},
					},
				},
			},
			"auto_patch_settings": schema.ListNestedBlock{
				Description: "Automatic patch updates of Portainer itself. Business Edition only. Sent only when the block is set.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"enabled":              ocBool("Whether Portainer applies patch releases automatically."),
					"patch_cron":           ocString("Cron expression of the patch window (e.g. \"0 4 * * *\")."),
					"portainer_repository": ocString("Image repository the Portainer patch images are pulled from (e.g. \"portainer/portainer-ee\")."),
					"registry_id":          ocInt("Identifier of the Portainer registry the patch images are pulled from. 0 means Docker Hub."),
					"updater_repository":   ocString("Image repository of the Portainer updater (e.g. \"portainer/portainer-updater\")."),
				}},
			},
			"edge": schema.ListNestedBlock{
				Description: "Default Edge agent settings: async mode intervals, the tunnel server address and mTLS. Business Edition only. Sent only when the block is set.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"async_mode":            ocBool("Whether new Edge agents run in async mode by default. Deprecated by Portainer since 2.18."),
						"command_interval":      ocInt("Command list interval of async Edge agents, in seconds."),
						"ping_interval":         ocInt("Ping interval of async Edge agents, in seconds."),
						"snapshot_interval":     ocInt("Snapshot interval of async Edge agents, in seconds."),
						"tunnel_server_address": ocString("Address where Edge agents reach Portainer's tunnel server (e.g. \"portainer.example.com:8000\")."),
					},
					Blocks: map[string]schema.Block{
						"mtls": schema.ListNestedBlock{
							Description: "mTLS between Portainer and Edge agents. The certificates are sent when set and never read back.",
							Validators:  maxOne(),
							NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
								"use_separate_cert": ocBool("Whether Edge agents use a separate certificate instead of Portainer's own TLS certificate."),
								"ca_cert":           schema.StringAttribute{Optional: true, Description: "PEM-encoded CA certificate that signs the Edge agent certificates. Portainer never returns it."},
								"cert":              schema.StringAttribute{Optional: true, Description: "PEM-encoded mTLS certificate. Portainer never returns it."},
								"key":               mtlsKey,
							}},
						},
					},
				},
			},
			"oidc_provider": schema.ListNestedBlock{
				Description: "Portainer's built-in OIDC provider, which signs users in to the clients of `portainer_oidc_client` and to add-ons. Business Edition 3.1 or later. Sent only when the block is set.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"public_url":                      ocString("URL browsers and add-ons reach Portainer at, such as the portal gateway (e.g. \"https://portainer.example.com\"). The OIDC issuer is this URL followed by `/api/oidc`. An empty string turns the OIDC provider off."),
					"access_token_lifetime":           settingsOIDCDuration("How long the provider's access and ID tokens last, as a Go duration (e.g. \"15m\"). An empty string restores Portainer's default, 15m."),
					"refresh_token_idle_lifetime":     settingsOIDCDuration("How long a session lasts unused, as a Go duration (e.g. \"24h\"). An empty string restores Portainer's default, 24h."),
					"refresh_token_absolute_lifetime": settingsOIDCDuration("How long a session lasts however it is used, as a Go duration (e.g. \"168h\"). An empty string restores Portainer's default, 168h."),
				}},
			},
			"ldap_settings": schema.ListNestedBlock{
				Description: "Configuration for the LDAP authentication provider.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"anonymous_mode":    ocBool("Whether to bind to the LDAP server anonymously instead of using reader credentials."),
						"auto_create_users": ocBool("Whether Portainer automatically creates a local user record on the first LDAP login."),
						"password":          ldapPassword,
						"reader_dn":         ocString("Distinguished Name of the LDAP account used to perform user lookups."),
						"start_tls":         ocBool("Whether to upgrade the LDAP connection to TLS using StartTLS."),
						"url":               ocString("URL of the LDAP server (e.g. \"ldap://ldap.example.com:389\")."),
					},
					Blocks: map[string]schema.Block{
						"search_settings": schema.ListNestedBlock{
							Description: "List of LDAP user search configurations applied to locate user entries.",
							NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
								"base_dn":             ocString("Base DN under which user entries are searched."),
								"filter":              ocString("LDAP search filter applied when looking up users."),
								"user_name_attribute": ocString("LDAP attribute used as the user's login name (e.g. \"uid\", \"sAMAccountName\")."),
							}},
						},
						"group_search_settings": schema.ListNestedBlock{
							Description: "List of LDAP group search configurations used to resolve user group memberships.",
							NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
								"group_attribute": ocString("LDAP attribute on group entries that lists their members."),
								"group_base_dn":   ocString("Base DN under which group entries are searched."),
								"group_filter":    ocString("LDAP search filter applied when looking up groups."),
							}},
						},
						"tls_config": schema.ListNestedBlock{
							Description: "TLS configuration used for the LDAP connection.",
							Validators:  maxOne(),
							NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
								"tls":             ocBool("Whether TLS is enabled for the LDAP connection."),
								"tls_ca_cert":     ocString("PEM-encoded CA certificate used to verify the LDAP server certificate."),
								"tls_cert":        ocString("PEM-encoded client certificate presented to the LDAP server."),
								"tls_key":         tlsKey,
								"tls_skip_verify": ocBool("Whether to skip verification of the LDAP server certificate."),
							}},
						},
					},
				},
			},
		},
	}
}

func (r *settingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ValidateConfig requires the block a top-level write-only secret belongs
// to, as SDKv2's RequiredWith did.
func (r *settingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, pair := range [][2]string{{"oauth_client_secret_wo", "oauth_settings"}, {"ldap_password_wo", "ldap_settings"}} {
		var wo types.String
		var block types.List
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(pair[0]), &wo)...)
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(pair[1]), &block)...)
		if wo.IsNull() || block.IsUnknown() {
			continue
		}
		if block.IsNull() || len(block.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(path.Root(pair[0]), "Missing required argument",
				fmt.Sprintf("%q: all of `%s,%s_version,%s` must be specified", pair[0], pair[0], pair[0], pair[1]))
		}
	}
}

// settingsBlockSecret is the secret of the oauth_settings or ldap_settings
// block, or the top-level write-only attribute woKey when its version is set.
// The write-only attributes cannot live inside the blocks: those are
// computed, and Terraform does not allow write-only attributes there.
//
// A configured write-only value wins, as in secretValue: the block is
// computed, so the block secret may still hold an old secret from state.
func settingsBlockSecret(ctx context.Context, config tfsdk.Config, block types.String, woKey string, version types.Int64, diags *diag.Diagnostics) string {
	if version.ValueInt64() != 0 {
		if wo := writeOnlyConfigString(ctx, config, path.Root(woKey), diags); wo != "" {
			return wo
		}
	}
	return block.ValueString()
}

// settingsConfiguredBool returns the configured value of an optional boolean, or nil
// when the configuration leaves it out so Portainer keeps its own.
func settingsConfiguredBool(ctx context.Context, config tfsdk.Config, key string, diags *diag.Diagnostics) *bool {
	var v types.Bool
	diags.Append(config.GetAttribute(ctx, path.Root(key), &v)...)
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func int64sToInts(ctx context.Context, l types.List, diags *diag.Diagnostics) []int {
	if l.IsNull() || l.IsUnknown() || len(l.Elements()) == 0 {
		return nil
	}
	return int64ListToInts(ctx, l, diags)
}

func stringsFromList(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// settingsKnownBool, settingsKnownString and settingsKnownInt64 return a
// planned value to send, nil when it is null or not known yet (an unset
// optional+computed attribute on create), so Portainer keeps its own.
func settingsKnownBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueBoolPointer()
}

func settingsKnownString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueStringPointer()
}

func settingsKnownInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueInt64Pointer()
}

// settingsEditionPayload adds the Business Edition settings the plan
// configures to request. custom_login_banner is optional and computed, so
// whether it is sent depends on the configuration, not on the plan.
func settingsEditionPayload(ctx context.Context, config tfsdk.Config, m *settingsModel, request *settingsUpdateRequest, diags *diag.Diagnostics) {
	var banner types.String
	diags.Append(config.GetAttribute(ctx, path.Root("custom_login_banner"), &banner)...)
	if !banner.IsNull() {
		request.CustomLoginBanner = settingsKnownString(m.CustomLoginBanner)
	}
	if len(m.AutoPatchSettings) > 0 {
		a := m.AutoPatchSettings[0]
		request.AutoPatchSettings = &settingsAutoPatchPayload{
			Enabled:             settingsKnownBool(a.Enabled),
			PatchCron:           settingsKnownString(a.PatchCron),
			PortainerRepository: settingsKnownString(a.PortainerRepository),
			RegistryID:          settingsKnownInt64(a.RegistryID),
			UpdaterRepository:   settingsKnownString(a.UpdaterRepository),
		}
	}
	if len(m.OIDCProvider) > 0 {
		o := m.OIDCProvider[0]
		request.OIDCProviderPublicURL = settingsKnownString(o.PublicURL)
		request.OIDCProviderAccessTokenLifetime = settingsKnownString(o.AccessTokenLifetime)
		request.OIDCProviderRefreshTokenIdleLifetime = settingsKnownString(o.RefreshTokenIdleLifetime)
		request.OIDCProviderRefreshTokenAbsoluteLifetime = settingsKnownString(o.RefreshTokenAbsoluteLifetime)
	}
	if len(m.Edge) > 0 {
		e := m.Edge[0]
		request.Edge = &settingsEdgePayload{
			AsyncMode:           settingsKnownBool(e.AsyncMode),
			CommandInterval:     settingsKnownInt64(e.CommandInterval),
			PingInterval:        settingsKnownInt64(e.PingInterval),
			SnapshotInterval:    settingsKnownInt64(e.SnapshotInterval),
			TunnelServerAddress: settingsKnownString(e.TunnelServerAddress),
		}
		if len(e.MTLS) > 0 {
			t := e.MTLS[0]
			request.Edge.MTLS = &settingsMTLSPayload{
				UseSeparateCert: settingsKnownBool(t.UseSeparateCert),
				CaCert:          settingsKnownString(t.CaCert),
				Cert:            settingsKnownString(t.Cert),
				Key:             settingsKnownString(t.Key),
			}
		}
	}
}

// payload builds PUT /settings from the plan.
func (r *settingsResource) payload(ctx context.Context, config tfsdk.Config, m *settingsModel, diags *diag.Diagnostics) settingsUpdateRequest {
	var internalAuth *InternalAuthSettings
	if len(m.InternalAuthSettings) > 0 {
		internalAuth = &InternalAuthSettings{RequiredPasswordLength: int(m.InternalAuthSettings[0].RequiredPasswordLength.ValueInt64())}
	}

	var oauth *OAuthSettings
	if len(m.OAuthSettings) > 0 {
		o := m.OAuthSettings[0]
		oauth = &OAuthSettings{
			AccessTokenURI:              o.AccessTokenURI.ValueString(),
			AuthStyle:                   int(o.AuthStyle.ValueInt64()),
			AuthorizationURI:            o.AuthorizationURI.ValueString(),
			ClientID:                    o.ClientID.ValueString(),
			ClientSecret:                settingsBlockSecret(ctx, config, o.ClientSecret, "oauth_client_secret_wo", m.OAuthClientSecretWOVersion, diags),
			DefaultTeamID:               int(o.DefaultTeamID.ValueInt64()),
			LogoutURI:                   o.LogoutURI.ValueString(),
			OAuthAutoCreateUsers:        o.OAuthAutoCreateUsers.ValueBool(),
			RedirectURI:                 o.RedirectURI.ValueString(),
			ResourceURI:                 o.ResourceURI.ValueString(),
			SSO:                         o.SSO.ValueBool(),
			Scopes:                      o.Scopes.ValueString(),
			UserIdentifier:              o.UserIdentifier.ValueString(),
			KubeSecretKey:               int64sToInts(ctx, o.KubeSecretKey, diags),
			HideInternalAuth:            o.HideInternalAuth.ValueBool(),
			MicrosoftTenantID:           o.MicrosoftTenantID.ValueString(),
			OAuthAutoMapTeamMemberships: o.OAuthAutoMapTeamMemberships.ValueBool(),
		}
		if len(o.TeamMemberships) > 0 {
			t := o.TeamMemberships[0]
			tm := &OAuthTeamMemberships{
				OAuthClaimName:            t.OAuthClaimName.ValueString(),
				AdminAutoPopulate:         t.AdminAutoPopulate.ValueBool(),
				AdminGroupClaimsRegexList: stringsFromList(ctx, t.AdminGroupClaimsRegexList, diags),
			}
			for _, mm := range t.OAuthClaimMappings {
				tm.OAuthClaimMappings = append(tm.OAuthClaimMappings, OAuthClaimMapping{
					ClaimValRegex: mm.ClaimValRegex.ValueString(),
					Team:          int(mm.Team.ValueInt64()),
				})
			}
			oauth.TeamMemberships = tm
		}
	}

	var ldap *LDAPSettings
	if len(m.LDAPSettings) > 0 {
		l := m.LDAPSettings[0]
		ldap = &LDAPSettings{
			AnonymousMode:   l.AnonymousMode.ValueBool(),
			AutoCreateUsers: l.AutoCreateUsers.ValueBool(),
			Password:        settingsBlockSecret(ctx, config, l.Password, "ldap_password_wo", m.LDAPPasswordWOVersion, diags),
			ReaderDN:        l.ReaderDN.ValueString(),
			StartTLS:        l.StartTLS.ValueBool(),
			URL:             l.URL.ValueString(),
		}
		for _, s := range l.SearchSettings {
			ldap.SearchSettings = append(ldap.SearchSettings, SearchSetting{
				BaseDN:            s.BaseDN.ValueString(),
				Filter:            s.Filter.ValueString(),
				UserNameAttribute: s.UserNameAttribute.ValueString(),
			})
		}
		for _, s := range l.GroupSearchSettings {
			ldap.GroupSearchSettings = append(ldap.GroupSearchSettings, GroupSearch{
				GroupAttribute: s.GroupAttribute.ValueString(),
				GroupBaseDN:    s.GroupBaseDN.ValueString(),
				GroupFilter:    s.GroupFilter.ValueString(),
			})
		}
		if len(l.TLSConfig) > 0 {
			t := l.TLSConfig[0]
			ldap.TLSConfig = &TLSConfig{
				TLS:           t.TLS.ValueBool(),
				TLSCACert:     t.TLSCACert.ValueString(),
				TLSCert:       t.TLSCert.ValueString(),
				TLSKey:        t.TLSKey.ValueString(),
				TLSSkipVerify: t.TLSSkipVerify.ValueBool(),
			}
		}
	}

	labels := make([]LabelPair, 0, len(m.BlackListedLabels))
	for _, l := range m.BlackListedLabels {
		labels = append(labels, LabelPair{Name: l.Name.ValueString(), Value: l.Value.ValueString()})
	}

	var globalOpts *GlobalDeploymentOpts
	if len(m.GlobalDeploymentOptions) > 0 {
		globalOpts = &GlobalDeploymentOpts{HideStacksFunctionality: m.GlobalDeploymentOptions[0].HideStacksFunctionality.ValueBool()}
	}

	request := settingsUpdateRequest{
		SettingsPayload: SettingsPayload{
			EdgePortainerURL:          m.EdgePortainerURL.ValueString(),
			AuthenticationMethod:      int(m.AuthenticationMethod.ValueInt64()),
			LogoURL:                   m.LogoURL.ValueString(),
			SnapshotInterval:          m.SnapshotInterval.ValueString(),
			TemplatesURL:              m.TemplatesURL.ValueString(),
			EnableEdgeComputeFeatures: m.EnableEdgeComputeFeatures.ValueBool(),
			EnforceEdgeID:             m.EnforceEdgeID.ValueBool(),
			UserSessionTimeout:        m.UserSessionTimeout.ValueString(),
			KubeconfigExpiry:          m.KubeconfigExpiry.ValueString(),
			KubectlShellImage:         m.KubectlShellImage.ValueString(),
			HelmRepositoryURL:         m.HelmRepositoryURL.ValueString(),
			AddonsCatalogURL:          m.AddonsCatalogURL.ValueString(),
			TrustOnFirstConnect:       m.TrustOnFirstConnect.ValueBool(),
			EdgeAgentCheckinInterval:  int(m.EdgeAgentCheckinInterval.ValueInt64()),
			BlackListedLabels:         labels,
			GlobalDeploymentOptions:   globalOpts,
			InternalAuthSettings:      internalAuth,
			OAuthSettings:             oauth,
			LDAPSettings:              ldap,
		},
		EnableEdgeComputeFeatures: settingsConfiguredBool(ctx, config, "enable_edge_compute_features", diags),
		EnforceEdgeID:             settingsConfiguredBool(ctx, config, "enforce_edge_id", diags),
		TrustOnFirstConnect:       settingsConfiguredBool(ctx, config, "trust_on_first_connect", diags),
		DisableKubeRolesSync:      settingsConfiguredBool(ctx, config, "disable_kube_roles_sync", diags),
		DisableKubeShell:          settingsConfiguredBool(ctx, config, "disable_kube_shell", diags),
		DisableKubeconfigDownload: settingsConfiguredBool(ctx, config, "disable_kubeconfig_download", diags),
		ForceSecureCookies:        settingsConfiguredBool(ctx, config, "force_secure_cookies", diags),
	}
	settingsEditionPayload(ctx, config, m, &request, diags)
	return request
}

func (r *settingsResource) get(ctx context.Context) (settingsResponse, error) {
	var result settingsResponse
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/settings", r.client.Endpoint), nil, &result); err != nil {
		return result, fmt.Errorf("failed to read settings: %w", err)
	}
	return result, nil
}

func listOfInts(values []int) types.List { return int64ListValue(values) }

func listOfStrings(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

// settingsReadMode tells settingsFromAPI which blocks to read back.
// Terraform plans as many blocks as the configuration has, so a block read
// back that the configuration leaves out would plan its removal on every run.
type settingsReadMode int

const (
	// settingsReadAll reads every block back (after an apply, where only the
	// unknown values of the plan are taken from it).
	settingsReadAll settingsReadMode = iota
	// settingsReadFramework reads back the blocks the state has: the
	// framework stored the configured ones.
	settingsReadFramework
	// settingsReadLegacy is for a state v3.0.0 wrote, or an import: it holds
	// every block Portainer reported, configured or not. A block counts as
	// configured when it differs from what an unconfigured Portainer reports.
	settingsReadLegacy
)

func (mode settingsReadMode) keep(prior int, configured bool) bool {
	switch mode {
	case settingsReadFramework:
		return prior > 0
	case settingsReadLegacy:
		return configured
	}
	return true
}

// settingsOAuthConfigured: an unconfigured Portainer reports OAuth settings
// without any endpoint, client or claim.
func settingsOAuthConfigured(o *OAuthSettings) bool {
	return o.AccessTokenURI != "" || o.AuthorizationURI != "" || o.ClientID != "" || o.ClientSecret != "" ||
		o.LogoutURI != "" || o.RedirectURI != "" || o.ResourceURI != "" || o.Scopes != "" || o.UserIdentifier != "" ||
		o.MicrosoftTenantID != "" || o.AuthStyle != 0 || o.DefaultTeamID != 0 || o.OAuthAutoCreateUsers ||
		o.OAuthAutoMapTeamMemberships || (o.TeamMemberships != nil && settingsTeamMembershipsConfigured(o.TeamMemberships))
}

func settingsTeamMembershipsConfigured(t *OAuthTeamMemberships) bool {
	return t.OAuthClaimName != "" || t.AdminAutoPopulate || len(t.AdminGroupClaimsRegexList) > 0 || len(t.OAuthClaimMappings) > 0
}

// settingsLDAPConfigured: an unconfigured Portainer reports anonymous LDAP
// settings that create users, without a server, an account or searches.
func settingsLDAPConfigured(l *LDAPSettings) bool {
	return l.ReaderDN != "" || l.URL != "" || l.Password != "" || l.StartTLS || !l.AnonymousMode || !l.AutoCreateUsers ||
		settingsSearchConfigured(l.SearchSettings) || settingsGroupSearchConfigured(l.GroupSearchSettings) ||
		(l.TLSConfig != nil && *l.TLSConfig != TLSConfig{})
}

func settingsSearchConfigured(entries []SearchSetting) bool {
	for _, e := range entries {
		if e != (SearchSetting{}) {
			return true
		}
	}
	return false
}

func settingsGroupSearchConfigured(entries []GroupSearch) bool {
	for _, e := range entries {
		if e != (GroupSearch{}) {
			return true
		}
	}
	return false
}

// settingsFromAPI is the state built from GET /settings; mode tells which
// blocks are read back. Secrets Portainer does not return keep the value
// from prior.
func settingsFromAPI(response settingsResponse, prior *settingsModel, mode settingsReadMode) settingsModel {
	result := response.SettingsPayload
	m := settingsModel{
		ID:                          types.StringValue(settingsID),
		EdgePortainerURL:            types.StringValue(result.EdgePortainerURL),
		AuthenticationMethod:        types.Int64Value(int64(result.AuthenticationMethod)),
		LogoURL:                     types.StringValue(result.LogoURL),
		SnapshotInterval:            types.StringValue(result.SnapshotInterval),
		TemplatesURL:                types.StringValue(result.TemplatesURL),
		EnableEdgeComputeFeatures:   types.BoolValue(result.EnableEdgeComputeFeatures),
		EnforceEdgeID:               types.BoolValue(result.EnforceEdgeID),
		UserSessionTimeout:          types.StringValue(result.UserSessionTimeout),
		KubeconfigExpiry:            types.StringValue(result.KubeconfigExpiry),
		KubectlShellImage:           types.StringValue(result.KubectlShellImage),
		AddonsCatalogURL:            types.StringValue(result.AddonsCatalogURL),
		HelmRepositoryURL:           types.StringValue(result.HelmRepositoryURL),
		DisableKubeRolesSync:        types.BoolValue(result.DisableKubeRolesSync),
		DisableKubeShell:            types.BoolValue(result.DisableKubeShell),
		DisableKubeconfigDownload:   types.BoolValue(result.DisableKubeconfigDownload),
		DisplayDonationHeader:       types.BoolValue(result.DisplayDonationHeader),
		DisplayExternalContributors: types.BoolValue(result.DisplayExternalContributors),
		IsDockerDesktopExtension:    types.BoolValue(result.IsDockerDesktopExtension),
		ForceSecureCookies:          types.BoolValue(result.ForceSecureCookies),
		TrustOnFirstConnect:         types.BoolValue(result.TrustOnFirstConnect),
		EdgeAgentCheckinInterval:    types.Int64Value(int64(result.EdgeAgentCheckinInterval)),
		OAuthClientSecretWO:         types.StringNull(),
		LDAPPasswordWO:              types.StringNull(),
		OAuthClientSecretWOVersion:  prior.OAuthClientSecretWOVersion,
		LDAPPasswordWOVersion:       prior.LDAPPasswordWOVersion,
		BlackListedLabels:           []settingsLabelModel{},
		GlobalDeploymentOptions:     []settingsGlobalOptsModel{},
		InternalAuthSettings:        []settingsInternalAuthModel{},
		OAuthSettings:               []settingsOAuthModel{},
		LDAPSettings:                []settingsLDAPSettingsModel{},
		CustomLoginBanner:           types.StringValue(response.CustomLoginBanner),
	}
	settingsEditionFromAPI(response, prior, mode, &m)

	if mode.keep(len(prior.BlackListedLabels), len(result.BlackListedLabels) > 0) {
		for _, l := range result.BlackListedLabels {
			m.BlackListedLabels = append(m.BlackListedLabels, settingsLabelModel{Name: types.StringValue(l.Name), Value: types.StringValue(l.Value)})
		}
	}

	// internal_auth_settings and global_deployment_options keep the state
	// value when Portainer reports none, as before.
	if ia := result.InternalAuthSettings; mode.keep(len(prior.InternalAuthSettings), ia != nil && ia.RequiredPasswordLength != 0 && ia.RequiredPasswordLength != 12) {
		m.InternalAuthSettings = prior.InternalAuthSettings
		if result.InternalAuthSettings != nil {
			m.InternalAuthSettings = []settingsInternalAuthModel{{RequiredPasswordLength: types.Int64Value(int64(result.InternalAuthSettings.RequiredPasswordLength))}}
		}
	}
	if g := result.GlobalDeploymentOptions; mode.keep(len(prior.GlobalDeploymentOptions), g != nil && g.HideStacksFunctionality) {
		m.GlobalDeploymentOptions = prior.GlobalDeploymentOptions
		if result.GlobalDeploymentOptions != nil {
			m.GlobalDeploymentOptions = []settingsGlobalOptsModel{{HideStacksFunctionality: types.BoolValue(result.GlobalDeploymentOptions.HideStacksFunctionality)}}
		}
	}
	if m.InternalAuthSettings == nil {
		m.InternalAuthSettings = []settingsInternalAuthModel{}
	}
	if m.GlobalDeploymentOptions == nil {
		m.GlobalDeploymentOptions = []settingsGlobalOptsModel{}
	}

	if o := result.OAuthSettings; o != nil && mode.keep(len(prior.OAuthSettings), settingsOAuthConfigured(o)) {
		var p settingsOAuthModel
		if len(prior.OAuthSettings) > 0 {
			p = prior.OAuthSettings[0]
		}
		oauth := settingsOAuthModel{
			AccessTokenURI:              types.StringValue(o.AccessTokenURI),
			AuthStyle:                   types.Int64Value(int64(o.AuthStyle)),
			AuthorizationURI:            types.StringValue(o.AuthorizationURI),
			ClientID:                    types.StringValue(o.ClientID),
			ClientSecret:                types.StringValue(p.ClientSecret.ValueString()),
			DefaultTeamID:               types.Int64Value(int64(o.DefaultTeamID)),
			LogoutURI:                   types.StringValue(o.LogoutURI),
			OAuthAutoCreateUsers:        types.BoolValue(o.OAuthAutoCreateUsers),
			RedirectURI:                 types.StringValue(o.RedirectURI),
			ResourceURI:                 types.StringValue(o.ResourceURI),
			SSO:                         types.BoolValue(o.SSO),
			Scopes:                      types.StringValue(o.Scopes),
			UserIdentifier:              types.StringValue(o.UserIdentifier),
			KubeSecretKey:               listOfInts(o.KubeSecretKey),
			HideInternalAuth:            types.BoolValue(o.HideInternalAuth),
			MicrosoftTenantID:           types.StringValue(o.MicrosoftTenantID),
			OAuthAutoMapTeamMemberships: types.BoolValue(o.OAuthAutoMapTeamMemberships),
			TeamMemberships:             []settingsTeamMembershipModel{},
		}
		if t := o.TeamMemberships; t != nil && mode.keep(len(p.TeamMemberships), settingsTeamMembershipsConfigured(t)) {
			var pt settingsTeamMembershipModel
			if len(p.TeamMemberships) > 0 {
				pt = p.TeamMemberships[0]
			}
			tm := settingsTeamMembershipModel{
				OAuthClaimName:            types.StringValue(t.OAuthClaimName),
				AdminAutoPopulate:         types.BoolValue(t.AdminAutoPopulate),
				AdminGroupClaimsRegexList: listOfStrings(t.AdminGroupClaimsRegexList),
				OAuthClaimMappings:        []settingsClaimMappingModel{},
			}
			if mode.keep(len(pt.OAuthClaimMappings), len(t.OAuthClaimMappings) > 0) {
				for _, mm := range t.OAuthClaimMappings {
					tm.OAuthClaimMappings = append(tm.OAuthClaimMappings, settingsClaimMappingModel{
						ClaimValRegex: types.StringValue(mm.ClaimValRegex),
						Team:          types.Int64Value(int64(mm.Team)),
					})
				}
			}
			oauth.TeamMemberships = []settingsTeamMembershipModel{tm}
		}
		m.OAuthSettings = []settingsOAuthModel{oauth}
	}

	if l := result.LDAPSettings; l != nil && mode.keep(len(prior.LDAPSettings), settingsLDAPConfigured(l)) {
		var p settingsLDAPSettingsModel
		if len(prior.LDAPSettings) > 0 {
			p = prior.LDAPSettings[0]
		}
		ldap := settingsLDAPSettingsModel{
			AnonymousMode:       types.BoolValue(l.AnonymousMode),
			AutoCreateUsers:     types.BoolValue(l.AutoCreateUsers),
			Password:            types.StringValue(p.Password.ValueString()),
			ReaderDN:            types.StringValue(l.ReaderDN),
			StartTLS:            types.BoolValue(l.StartTLS),
			URL:                 types.StringValue(l.URL),
			SearchSettings:      []settingsLDAPSearchModel{},
			GroupSearchSettings: []settingsLDAPGroupModel{},
			TLSConfig:           []settingsLDAPTLSModel{},
		}
		if mode.keep(len(p.SearchSettings), settingsSearchConfigured(l.SearchSettings)) {
			for _, s := range l.SearchSettings {
				ldap.SearchSettings = append(ldap.SearchSettings, settingsLDAPSearchModel{
					BaseDN:            types.StringValue(s.BaseDN),
					Filter:            types.StringValue(s.Filter),
					UserNameAttribute: types.StringValue(s.UserNameAttribute),
				})
			}
		}
		if mode.keep(len(p.GroupSearchSettings), settingsGroupSearchConfigured(l.GroupSearchSettings)) {
			for _, s := range l.GroupSearchSettings {
				ldap.GroupSearchSettings = append(ldap.GroupSearchSettings, settingsLDAPGroupModel{
					GroupAttribute: types.StringValue(s.GroupAttribute),
					GroupBaseDN:    types.StringValue(s.GroupBaseDN),
					GroupFilter:    types.StringValue(s.GroupFilter),
				})
			}
		}
		if t := l.TLSConfig; t != nil && mode.keep(len(p.TLSConfig), *t != TLSConfig{}) {
			ldap.TLSConfig = []settingsLDAPTLSModel{{
				TLS:           types.BoolValue(t.TLS),
				TLSCACert:     types.StringValue(t.TLSCACert),
				TLSCert:       types.StringValue(t.TLSCert),
				TLSKey:        types.StringValue(t.TLSKey),
				TLSSkipVerify: types.BoolValue(t.TLSSkipVerify),
			}}
		}
		m.LDAPSettings = []settingsLDAPSettingsModel{ldap}
	}
	return m
}

// keepNew tells whether a block added in v3.1.0 is read back. No v3.0.0
// state or import has it, so unlike the older blocks it is only read back
// when the state has it or after an apply.
func (mode settingsReadMode) keepNew(prior int) bool {
	return mode == settingsReadAll || prior > 0
}

// settingsEditionFromAPI reads back the Business Edition blocks. The mTLS
// certificates are not returned (Portainer keeps only file paths), so they
// keep their prior value.
func settingsEditionFromAPI(response settingsResponse, prior *settingsModel, mode settingsReadMode, m *settingsModel) {
	m.AutoPatchSettings = []settingsAutoPatchModel{}
	if a := response.AutoPatchSettings; a != nil && mode.keepNew(len(prior.AutoPatchSettings)) {
		m.AutoPatchSettings = []settingsAutoPatchModel{{
			Enabled:             types.BoolValue(a.Enabled),
			PatchCron:           types.StringValue(a.PatchCron),
			PortainerRepository: types.StringValue(a.PortainerRepository),
			RegistryID:          types.Int64Value(a.RegistryID),
			UpdaterRepository:   types.StringValue(a.UpdaterRepository),
		}}
	}
	m.OIDCProvider = []settingsOIDCProviderModel{}
	if o := response.OIDCProviderSettings; o != nil && mode.keepNew(len(prior.OIDCProvider)) {
		m.OIDCProvider = []settingsOIDCProviderModel{{
			PublicURL:                    types.StringValue(o.PublicURL),
			AccessTokenLifetime:          types.StringValue(o.AccessTokenLifetime),
			RefreshTokenIdleLifetime:     types.StringValue(o.RefreshTokenIdleLifetime),
			RefreshTokenAbsoluteLifetime: types.StringValue(o.RefreshTokenAbsoluteLifetime),
		}}
	}
	m.Edge = []settingsEdgeModel{}
	e := response.Edge
	if e == nil || !mode.keepNew(len(prior.Edge)) {
		return
	}
	var p settingsEdgeModel
	if len(prior.Edge) > 0 {
		p = prior.Edge[0]
	}
	edge := settingsEdgeModel{
		AsyncMode:           types.BoolValue(e.AsyncMode),
		CommandInterval:     types.Int64Value(e.CommandInterval),
		PingInterval:        types.Int64Value(e.PingInterval),
		SnapshotInterval:    types.Int64Value(e.SnapshotInterval),
		TunnelServerAddress: types.StringValue(e.TunnelServerAddress),
		MTLS:                []settingsMTLSModel{},
	}
	if e.MTLS != nil && mode.keepNew(len(p.MTLS)) {
		pm := settingsMTLSModel{CaCert: types.StringNull(), Cert: types.StringNull(), Key: types.StringNull()}
		if len(p.MTLS) > 0 {
			pm = p.MTLS[0]
		}
		edge.MTLS = []settingsMTLSModel{{
			UseSeparateCert: types.BoolValue(e.MTLS.UseSeparateCert),
			CaCert:          pm.CaCert,
			Cert:            pm.Cert,
			Key:             pm.Key,
		}}
	}
	m.Edge = []settingsEdgeModel{edge}
}

func (r *settingsResource) apply(ctx context.Context, config tfsdk.Config, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m settingsModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	request := r.payload(ctx, config, &m, diags)
	if diags.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/settings", r.client.Endpoint), request, nil); err != nil {
		diags.AddError("Failed to update settings", fmt.Sprintf("failed to update settings: %s", err))
		return
	}

	// The configured values are what was just written. Values the plan
	// leaves unknown (computed ones on create) come from Portainer.
	result, err := r.get(ctx)
	if err != nil {
		diags.AddError("Failed to read settings", err.Error())
		return
	}
	read := tfsdk.State{Schema: state.Schema, Raw: tftypes.NewValue(state.Schema.Type().TerraformType(ctx), nil)}
	readModel := settingsFromAPI(result, &m, settingsReadAll)
	diags.Append(read.Set(ctx, &readModel)...)
	if diags.HasError() {
		return
	}
	filled, err := gsbFillUnknown(plan.Raw, read.Raw)
	if err != nil {
		diags.AddError("Failed to update settings", err.Error())
		return
	}
	state.Raw = filled
	diags.Append(state.SetAttribute(ctx, path.Root("id"), settingsID)...)
}

func (r *settingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *settingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.apply(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *settingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior settingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.get(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read settings", err.Error())
		return
	}
	mode := settingsReadLegacy
	if gsbFrameworkState(ctx, req.Private) {
		mode = settingsReadFramework
	}
	m := settingsFromAPI(result, &prior, mode)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the settings, which always exist.
func (r *settingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *settingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
