package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SettingsPayload struct {
	EdgePortainerURL            string                `json:"EdgePortainerURL,omitempty"`
	AuthenticationMethod        int                   `json:"authenticationMethod,omitempty"`
	EnableTelemetry             bool                  `json:"EnableTelemetry,omitempty"`
	LogoURL                     string                `json:"logoURL,omitempty"`
	SnapshotInterval            string                `json:"snapshotInterval,omitempty"`
	TemplatesURL                string                `json:"templatesURL,omitempty"`
	EnableEdgeComputeFeatures   bool                  `json:"enableEdgeComputeFeatures,omitempty"`
	EnforceEdgeID               bool                  `json:"enforceEdgeID,omitempty"`
	UserSessionTimeout          string                `json:"userSessionTimeout,omitempty"`
	KubeconfigExpiry            string                `json:"kubeconfigExpiry,omitempty"`
	KubectlShellImage           string                `json:"kubectlShellImage,omitempty"`
	HelmRepositoryURL           string                `json:"helmRepositoryURL,omitempty"`
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

type OAuthSettings struct {
	AccessTokenURI       string `json:"AccessTokenURI,omitempty"`
	AuthStyle            int    `json:"AuthStyle,omitempty"`
	AuthorizationURI     string `json:"AuthorizationURI,omitempty"`
	ClientID             string `json:"ClientID,omitempty"`
	ClientSecret         string `json:"ClientSecret,omitempty"`
	DefaultTeamID        int    `json:"DefaultTeamID,omitempty"`
	LogoutURI            string `json:"LogoutURI,omitempty"`
	OAuthAutoCreateUsers bool   `json:"OAuthAutoCreateUsers,omitempty"`
	RedirectURI          string `json:"RedirectURI,omitempty"`
	ResourceURI          string `json:"ResourceURI,omitempty"`
	SSO                  bool   `json:"SSO,omitempty"`
	Scopes               string `json:"Scopes,omitempty"`
	UserIdentifier       string `json:"UserIdentifier,omitempty"`
	KubeSecretKey        []int  `json:"KubeSecretKey,omitempty"`
}

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

func resourceSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceSettingsApply,
		Read:   resourceSettingsRead,
		Update: resourceSettingsApply,
		Delete: resourceSettingsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"edge_portainer_url":           {Type: schema.TypeString, Optional: true, Computed: true},
			"authentication_method":        {Type: schema.TypeInt, Optional: true, Computed: true},
			"enable_telemetry":             {Type: schema.TypeBool, Optional: true, Computed: true},
			"logo_url":                     {Type: schema.TypeString, Optional: true, Computed: true},
			"snapshot_interval":            {Type: schema.TypeString, Optional: true, Computed: true},
			"templates_url":                {Type: schema.TypeString, Optional: true, Computed: true},
			"enable_edge_compute_features": {Type: schema.TypeBool, Optional: true, Computed: true},
			"enforce_edge_id":              {Type: schema.TypeBool, Optional: true, Computed: true},
			"user_session_timeout":         {Type: schema.TypeString, Optional: true, Computed: true},
			"kubeconfig_expiry":            {Type: schema.TypeString, Optional: true, Computed: true},
			"kubectl_shell_image":          {Type: schema.TypeString, Optional: true, Computed: true},
			"helm_repository_url":          {Type: schema.TypeString, Optional: true, Computed: true},
			"disable_kube_roles_sync": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"disable_kube_shell": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"disable_kubeconfig_download": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"display_donation_header": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"display_external_contributors": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"is_docker_desktop_extension": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"trust_on_first_connect":      {Type: schema.TypeBool, Optional: true, Computed: true},
			"edge_agent_checkin_interval": {Type: schema.TypeInt, Optional: true, Computed: true},
			"black_listed_labels": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":  {Type: schema.TypeString, Required: true},
						"value": {Type: schema.TypeString, Required: true},
					},
				},
			},
			"global_deployment_options": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hide_stacks_functionality": {Type: schema.TypeBool, Optional: true, Computed: true},
					},
				},
			},
			"internal_auth_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"required_password_length": {Type: schema.TypeInt, Optional: true, Computed: true},
					},
				},
			},
			"oauth_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"access_token_uri":  {Type: schema.TypeString, Optional: true, Computed: true},
						"auth_style":        {Type: schema.TypeInt, Optional: true, Computed: true},
						"authorization_uri": {Type: schema.TypeString, Optional: true, Computed: true},
						"client_id":         {Type: schema.TypeString, Optional: true, Computed: true},
						"client_secret": {
							Type:      schema.TypeString,
							Optional:  true,
							Computed:  true,
							Sensitive: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return old == "" || new == ""
							},
						},
						"default_team_id":         {Type: schema.TypeInt, Optional: true, Computed: true},
						"logout_uri":              {Type: schema.TypeString, Optional: true, Computed: true},
						"oauth_auto_create_users": {Type: schema.TypeBool, Optional: true, Computed: true},
						"redirect_uri":            {Type: schema.TypeString, Optional: true, Computed: true},
						"resource_uri":            {Type: schema.TypeString, Optional: true, Computed: true},
						"sso":                     {Type: schema.TypeBool, Optional: true, Computed: true},
						"scopes":                  {Type: schema.TypeString, Optional: true, Computed: true},
						"user_identifier":         {Type: schema.TypeString, Optional: true, Computed: true},
						"kube_secret_key": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeInt},
						},
					},
				},
			},
			"ldap_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"anonymous_mode":    {Type: schema.TypeBool, Optional: true, Computed: true},
						"auto_create_users": {Type: schema.TypeBool, Optional: true, Computed: true},
						"password": {
							Type:      schema.TypeString,
							Optional:  true,
							Computed:  true,
							Sensitive: true,
							DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
								return old == "" || new == ""
							},
						},
						"reader_dn": {Type: schema.TypeString, Optional: true, Computed: true},
						"start_tls": {Type: schema.TypeBool, Optional: true, Computed: true},
						"url":       {Type: schema.TypeString, Optional: true, Computed: true},
						"search_settings": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"base_dn":             {Type: schema.TypeString, Optional: true, Computed: true},
									"filter":              {Type: schema.TypeString, Optional: true, Computed: true},
									"user_name_attribute": {Type: schema.TypeString, Optional: true, Computed: true},
								},
							},
						},
						"group_search_settings": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"group_attribute": {Type: schema.TypeString, Optional: true, Computed: true},
									"group_base_dn":   {Type: schema.TypeString, Optional: true, Computed: true},
									"group_filter":    {Type: schema.TypeString, Optional: true, Computed: true},
								},
							},
						},
						"tls_config": {
							Type:     schema.TypeList,
							Optional: true,
							Computed: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"tls":             {Type: schema.TypeBool, Optional: true, Computed: true},
									"tls_ca_cert":     {Type: schema.TypeString, Optional: true, Computed: true},
									"tls_cert":        {Type: schema.TypeString, Optional: true, Computed: true},
									"tls_key":         {Type: schema.TypeString, Optional: true, Computed: true},
									"tls_skip_verify": {Type: schema.TypeBool, Optional: true, Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func resourceSettingsApply(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// Internal auth parsing
	var internalAuth *InternalAuthSettings
	if v, ok := d.GetOk("internal_auth_settings"); ok {
		items := v.([]interface{})
		if len(items) > 0 && items[0] != nil {
			m := items[0].(map[string]interface{})
			internalAuth = &InternalAuthSettings{
				RequiredPasswordLength: m["required_password_length"].(int),
			}
		}
	}

	// OAuth settings parsing
	var oauth *OAuthSettings
	if v, ok := d.GetOk("oauth_settings"); ok {
		items := v.([]interface{})
		if len(items) > 0 && items[0] != nil {
			m := items[0].(map[string]interface{})
			oauth = &OAuthSettings{
				AccessTokenURI:       m["access_token_uri"].(string),
				AuthStyle:            m["auth_style"].(int),
				AuthorizationURI:     m["authorization_uri"].(string),
				ClientID:             m["client_id"].(string),
				ClientSecret:         m["client_secret"].(string),
				DefaultTeamID:        m["default_team_id"].(int),
				LogoutURI:            m["logout_uri"].(string),
				OAuthAutoCreateUsers: m["oauth_auto_create_users"].(bool),
				RedirectURI:          m["redirect_uri"].(string),
				ResourceURI:          m["resource_uri"].(string),
				SSO:                  m["sso"].(bool),
				Scopes:               m["scopes"].(string),
				UserIdentifier:       m["user_identifier"].(string),
			}
			if raw, ok := m["kube_secret_key"]; ok {
				for _, v := range raw.([]interface{}) {
					oauth.KubeSecretKey = append(oauth.KubeSecretKey, v.(int))
				}
			}
		}
	}

	// LDAP settings parsing
	var ldap *LDAPSettings
	if v, ok := d.GetOk("ldap_settings"); ok {
		items := v.([]interface{})
		if len(items) > 0 && items[0] != nil {
			m := items[0].(map[string]interface{})

			ldap = &LDAPSettings{
				AnonymousMode:   m["anonymous_mode"].(bool),
				AutoCreateUsers: m["auto_create_users"].(bool),
				Password:        m["password"].(string),
				ReaderDN:        m["reader_dn"].(string),
				StartTLS:        m["start_tls"].(bool),
				URL:             m["url"].(string),
			}

			if raw, ok := m["search_settings"]; ok {
				for _, v := range raw.([]interface{}) {
					s := v.(map[string]interface{})
					ldap.SearchSettings = append(ldap.SearchSettings, SearchSetting{
						BaseDN:            s["base_dn"].(string),
						Filter:            s["filter"].(string),
						UserNameAttribute: s["user_name_attribute"].(string),
					})
				}
			}

			if raw, ok := m["group_search_settings"]; ok {
				for _, v := range raw.([]interface{}) {
					s := v.(map[string]interface{})
					ldap.GroupSearchSettings = append(ldap.GroupSearchSettings, GroupSearch{
						GroupAttribute: s["group_attribute"].(string),
						GroupBaseDN:    s["group_base_dn"].(string),
						GroupFilter:    s["group_filter"].(string),
					})
				}
			}

			if raw, ok := m["tls_config"]; ok {
				tlsItems := raw.([]interface{})
				if len(tlsItems) > 0 && tlsItems[0] != nil {
					tlsMap := tlsItems[0].(map[string]interface{})
					ldap.TLSConfig = &TLSConfig{
						TLS:           tlsMap["tls"].(bool),
						TLSCACert:     tlsMap["tls_ca_cert"].(string),
						TLSCert:       tlsMap["tls_cert"].(string),
						TLSKey:        tlsMap["tls_key"].(string),
						TLSSkipVerify: tlsMap["tls_skip_verify"].(bool),
					}
				}
			}
		}
	}

	// Labels
	var labels []LabelPair
	if v, ok := d.GetOk("black_listed_labels"); ok {
		for _, raw := range v.([]interface{}) {
			item := raw.(map[string]interface{})
			labels = append(labels, LabelPair{
				Name:  item["name"].(string),
				Value: item["value"].(string),
			})
		}
	}

	// Global deployment options
	var globalOpts *GlobalDeploymentOpts
	if v, ok := d.GetOk("global_deployment_options"); ok {
		items := v.([]interface{})
		if len(items) > 0 && items[0] != nil {
			m := items[0].(map[string]interface{})
			globalOpts = &GlobalDeploymentOpts{
				HideStacksFunctionality: m["hide_stacks_functionality"].(bool),
			}
		}
	}

	payload := SettingsPayload{
		EdgePortainerURL:          d.Get("edge_portainer_url").(string),
		AuthenticationMethod:      d.Get("authentication_method").(int),
		EnableTelemetry:           d.Get("enable_telemetry").(bool),
		LogoURL:                   d.Get("logo_url").(string),
		SnapshotInterval:          d.Get("snapshot_interval").(string),
		TemplatesURL:              d.Get("templates_url").(string),
		EnableEdgeComputeFeatures: d.Get("enable_edge_compute_features").(bool),
		EnforceEdgeID:             d.Get("enforce_edge_id").(bool),
		UserSessionTimeout:        d.Get("user_session_timeout").(string),
		KubeconfigExpiry:          d.Get("kubeconfig_expiry").(string),
		KubectlShellImage:         d.Get("kubectl_shell_image").(string),
		HelmRepositoryURL:         d.Get("helm_repository_url").(string),
		TrustOnFirstConnect:       d.Get("trust_on_first_connect").(bool),
		EdgeAgentCheckinInterval:  d.Get("edge_agent_checkin_interval").(int),
		BlackListedLabels:         labels,
		GlobalDeploymentOptions:   globalOpts,
		InternalAuthSettings:      internalAuth,
		OAuthSettings:             oauth,
		LDAPSettings:              ldap,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/settings", client.Endpoint), bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to update settings: %s", string(body))
	}
	d.SetId("portainer-settings")
	return nil
}

func resourceSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/settings", client.Endpoint), nil)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to read settings, status: %d", resp.StatusCode)
	}

	var result SettingsPayload
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId("portainer-settings")
	_ = d.Set("edge_portainer_url", result.EdgePortainerURL)
	_ = d.Set("authentication_method", result.AuthenticationMethod)
	// _ = d.Set("enable_telemetry", result.EnableTelemetry)
	_ = d.Set("logo_url", result.LogoURL)
	_ = d.Set("snapshot_interval", result.SnapshotInterval)
	_ = d.Set("templates_url", result.TemplatesURL)
	_ = d.Set("enable_edge_compute_features", result.EnableEdgeComputeFeatures)
	_ = d.Set("enforce_edge_id", result.EnforceEdgeID)
	_ = d.Set("user_session_timeout", result.UserSessionTimeout)
	_ = d.Set("kubeconfig_expiry", result.KubeconfigExpiry)
	_ = d.Set("kubectl_shell_image", result.KubectlShellImage)
	_ = d.Set("helm_repository_url", result.HelmRepositoryURL)
	_ = d.Set("trust_on_first_connect", result.TrustOnFirstConnect)
	_ = d.Set("edge_agent_checkin_interval", result.EdgeAgentCheckinInterval)
	_ = d.Set("disable_kube_roles_sync", result.DisableKubeRolesSync)
	_ = d.Set("disable_kube_shell", result.DisableKubeShell)
	_ = d.Set("disable_kubeconfig_download", result.DisableKubeconfigDownload)
	_ = d.Set("display_donation_header", result.DisplayDonationHeader)
	_ = d.Set("display_external_contributors", result.DisplayExternalContributors)
	_ = d.Set("is_docker_desktop_extension", result.IsDockerDesktopExtension)

	// black_listed_labels
	labels := make([]map[string]interface{}, 0, len(result.BlackListedLabels))
	for _, label := range result.BlackListedLabels {
		labels = append(labels, map[string]interface{}{
			"name":  label.Name,
			"value": label.Value,
		})
	}
	_ = d.Set("black_listed_labels", labels)

	// internal_auth_settings
	if result.InternalAuthSettings != nil {
		d.Set("internal_auth_settings", []interface{}{map[string]interface{}{
			"required_password_length": result.InternalAuthSettings.RequiredPasswordLength,
		}})
	}

	// global_deployment_options
	if result.GlobalDeploymentOptions != nil {
		d.Set("global_deployment_options", []interface{}{map[string]interface{}{
			"hide_stacks_functionality": result.GlobalDeploymentOptions.HideStacksFunctionality,
		}})
	}

	// oauth_settings
	if result.OAuthSettings != nil {
		oauth := map[string]interface{}{
			"access_token_uri":        result.OAuthSettings.AccessTokenURI,
			"auth_style":              result.OAuthSettings.AuthStyle,
			"authorization_uri":       result.OAuthSettings.AuthorizationURI,
			"client_id":               result.OAuthSettings.ClientID,
			"client_secret":           result.OAuthSettings.ClientSecret,
			"default_team_id":         result.OAuthSettings.DefaultTeamID,
			"logout_uri":              result.OAuthSettings.LogoutURI,
			"oauth_auto_create_users": result.OAuthSettings.OAuthAutoCreateUsers,
			"redirect_uri":            result.OAuthSettings.RedirectURI,
			"resource_uri":            result.OAuthSettings.ResourceURI,
			"sso":                     result.OAuthSettings.SSO,
			"scopes":                  result.OAuthSettings.Scopes,
			"user_identifier":         result.OAuthSettings.UserIdentifier,
			"kube_secret_key":         result.OAuthSettings.KubeSecretKey,
		}
		d.Set("oauth_settings", []interface{}{oauth})
	}

	// ldap_settings
	if result.LDAPSettings != nil {
		ldap := map[string]interface{}{
			"anonymous_mode":    result.LDAPSettings.AnonymousMode,
			"auto_create_users": result.LDAPSettings.AutoCreateUsers,
			"password":          result.LDAPSettings.Password,
			"reader_dn":         result.LDAPSettings.ReaderDN,
			"start_tls":         result.LDAPSettings.StartTLS,
			"url":               result.LDAPSettings.URL,
		}

		// search_settings
		search := make([]interface{}, 0, len(result.LDAPSettings.SearchSettings))
		for _, s := range result.LDAPSettings.SearchSettings {
			search = append(search, map[string]interface{}{
				"base_dn":             s.BaseDN,
				"filter":              s.Filter,
				"user_name_attribute": s.UserNameAttribute,
			})
		}
		ldap["search_settings"] = search

		// group_search_settings
		groupSearch := make([]interface{}, 0, len(result.LDAPSettings.GroupSearchSettings))
		for _, s := range result.LDAPSettings.GroupSearchSettings {
			groupSearch = append(groupSearch, map[string]interface{}{
				"group_attribute": s.GroupAttribute,
				"group_base_dn":   s.GroupBaseDN,
				"group_filter":    s.GroupFilter,
			})
		}
		ldap["group_search_settings"] = groupSearch

		// tls_config
		if result.LDAPSettings.TLSConfig != nil {
			ldap["tls_config"] = []interface{}{map[string]interface{}{
				"tls":             result.LDAPSettings.TLSConfig.TLS,
				"tls_ca_cert":     result.LDAPSettings.TLSConfig.TLSCACert,
				"tls_cert":        result.LDAPSettings.TLSConfig.TLSCert,
				"tls_key":         result.LDAPSettings.TLSConfig.TLSKey,
				"tls_skip_verify": result.LDAPSettings.TLSConfig.TLSSkipVerify,
			}}
		}

		d.Set("ldap_settings", []interface{}{ldap})
	}

	return nil
}

func resourceSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
