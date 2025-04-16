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
	EdgePortainerURL          string                `json:"EdgePortainerURL,omitempty"`
	AuthenticationMethod      int                   `json:"authenticationMethod,omitempty"`
	EnableTelemetry           bool                  `json:"enableTelemetry,omitempty"`
	LogoURL                   string                `json:"logoURL,omitempty"`
	SnapshotInterval          string                `json:"snapshotInterval,omitempty"`
	TemplatesURL              string                `json:"templatesURL,omitempty"`
	EnableEdgeComputeFeatures bool                  `json:"enableEdgeComputeFeatures,omitempty"`
	EnforceEdgeID             bool                  `json:"enforceEdgeID,omitempty"`
	UserSessionTimeout        string                `json:"userSessionTimeout,omitempty"`
	KubeconfigExpiry          string                `json:"kubeconfigExpiry,omitempty"`
	KubectlShellImage         string                `json:"kubectlShellImage,omitempty"`
	HelmRepositoryURL         string                `json:"helmRepositoryURL,omitempty"`
	InternalAuthSettings      *InternalAuthSettings `json:"internalAuthSettings,omitempty"`
	OAuthSettings             *OAuthSettings        `json:"oauthSettings,omitempty"`
	LDAPSettings              *LDAPSettings         `json:"ldapsettings,omitempty"`
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
}

type LDAPSettings struct {
	AnonymousMode   bool   `json:"AnonymousMode,omitempty"`
	AutoCreateUsers bool   `json:"AutoCreateUsers,omitempty"`
	Password        string `json:"Password,omitempty"`
	ReaderDN        string `json:"ReaderDN,omitempty"`
	StartTLS        bool   `json:"StartTLS,omitempty"`
	URL             string `json:"URL,omitempty"`
}

func resourceSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceSettingsApply,
		Read:   resourceSettingsRead,
		Update: resourceSettingsApply,
		Delete: resourceSettingsDelete,
		Schema: map[string]*schema.Schema{
			"edge_portainer_url":           {Type: schema.TypeString, Optional: true},
			"authentication_method":        {Type: schema.TypeInt, Optional: true},
			"enable_telemetry":             {Type: schema.TypeBool, Optional: true},
			"logo_url":                     {Type: schema.TypeString, Optional: true},
			"snapshot_interval":            {Type: schema.TypeString, Optional: true},
			"templates_url":                {Type: schema.TypeString, Optional: true},
			"enable_edge_compute_features": {Type: schema.TypeBool, Optional: true},
			"enforce_edge_id":              {Type: schema.TypeBool, Optional: true},
			"user_session_timeout":         {Type: schema.TypeString, Optional: true},
			"kubeconfig_expiry":            {Type: schema.TypeString, Optional: true},
			"kubectl_shell_image":          {Type: schema.TypeString, Optional: true},
			"helm_repository_url":          {Type: schema.TypeString, Optional: true},
			"internal_auth_settings": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"required_password_length": {Type: schema.TypeInt, Optional: true},
					},
				},
			},
			"oauth_settings": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"access_token_uri":        {Type: schema.TypeString, Optional: true},
						"auth_style":              {Type: schema.TypeInt, Optional: true},
						"authorization_uri":       {Type: schema.TypeString, Optional: true},
						"client_id":               {Type: schema.TypeString, Optional: true},
						"client_secret":           {Type: schema.TypeString, Optional: true, Sensitive: true},
						"default_team_id":         {Type: schema.TypeInt, Optional: true},
						"logout_uri":              {Type: schema.TypeString, Optional: true},
						"oauth_auto_create_users": {Type: schema.TypeBool, Optional: true},
						"redirect_uri":            {Type: schema.TypeString, Optional: true},
						"resource_uri":            {Type: schema.TypeString, Optional: true},
						"sso":                     {Type: schema.TypeBool, Optional: true},
						"scopes":                  {Type: schema.TypeString, Optional: true},
						"user_identifier":         {Type: schema.TypeString, Optional: true},
					},
				},
			},
			"ldap_settings": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"anonymous_mode":    {Type: schema.TypeBool, Optional: true},
						"auto_create_users": {Type: schema.TypeBool, Optional: true},
						"password":          {Type: schema.TypeString, Optional: true, Sensitive: true},
						"reader_dn":         {Type: schema.TypeString, Optional: true},
						"start_tls":         {Type: schema.TypeBool, Optional: true},
						"url":               {Type: schema.TypeString, Optional: true},
					},
				},
			},
		},
	}
}

func resourceSettingsApply(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

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
		}
	}

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
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
	return nil
}

func resourceSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
