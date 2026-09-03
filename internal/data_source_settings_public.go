package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceSettingsPublic() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceSettingsPublicRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"authentication_method": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Authentication method in use: 1 = internal, 2 = LDAP, 3 = OAuth.",
			},
			"enable_edge_compute_features": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Edge Compute features are enabled, which Edge environments and Edge stacks require.",
			},
			"required_password_length": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Minimum password length Portainer enforces for internal accounts.",
			},
			"requires_setup_token": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the instance still demands a setup token, meaning it has not been initialised.",
			},
			"kubeconfig_expiry": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Lifetime of generated kubeconfig files as a duration string, `0` meaning they never expire.",
			},
			"logo_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Custom logo URL shown in the UI, empty when Portainer's own logo is used.",
			},
			"oauth_login_uri": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URI users are sent to for OAuth login, empty when OAuth is not configured.",
			},
			"oauth_logout_uri": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URI users are sent to on OAuth logout.",
			},
			"team_sync": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether team membership is synchronised from the external authentication provider.",
			},
			"is_docker_desktop_extension": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether this Portainer runs as the Docker Desktop extension.",
			},
		},
	}
}

func dataSourceSettingsPublicRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// This endpoint needs no authentication, which is what makes it usable to
	// discover how to authenticate in the first place.
	var settings struct {
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
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/settings/public", nil, &settings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the public Portainer settings: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"authentication_method":        settings.AuthenticationMethod,
		"enable_edge_compute_features": settings.EnableEdgeComputeFeatures,
		"required_password_length":     settings.RequiredPasswordLength,
		"requires_setup_token":         settings.RequiresSetupToken,
		"kubeconfig_expiry":            settings.KubeconfigExpiry,
		"logo_url":                     settings.LogoURL,
		"oauth_login_uri":              settings.OAuthLoginURI,
		"oauth_logout_uri":             settings.OAuthLogoutURI,
		"team_sync":                    settings.TeamSync,
		"is_docker_desktop_extension":  settings.IsDockerDesktopExtension,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-public-settings")
	return nil
}
