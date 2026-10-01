package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceRegistryConnection() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRegistryConnectionRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the registry to test, without a scheme for most types (for example `registry.example.com`).",
			},
			"type": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntBetween(1, 8),
				Description:  "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR, 8 = GitHub.",
			},
			// Portainer rejects a ping without credentials with 400 "Username and
			// password are required", for every registry type including DockerHub.
			// Requiring them here turns that into a plan-time error naming the
			// attribute, rather than a confusing failure half way through an apply.
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Username used for the test. Portainer requires credentials for every registry type, including public ones.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password or token paired with `username`.",
			},
			"tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the registry is contacted over TLS.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether an unreachable registry makes the data source itself fail. Defaults to false, which reports the outcome in `success` and `message`.",
			},
			// Computed attributes
			"success": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach and authenticate against the registry.",
			},
			"message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Message returned by Portainer describing the outcome, carrying the reason on failure.",
			},
		},
	}
}

func dataSourceRegistryConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// registries.registryPingPayload uses PascalCase field names.
	payload := map[string]interface{}{
		"URL":  d.Get("url").(string),
		"Type": d.Get("type").(int),
		"TLS":  d.Get("tls").(bool),
	}
	payload["Username"] = d.Get("username").(string)
	payload["Password"] = d.Get("password").(string)

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/registries/ping", payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to test the registry connection: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	}); err != nil {
		return diag.FromErr(err)
	}

	// The request itself succeeded even when the registry is unreachable, so the
	// ID is set before the opt-in failure below.
	d.SetId("registry-ping-" + d.Get("url").(string))

	if !result.Success && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the registry connection test failed: %s", result.Message))
	}

	return nil
}
