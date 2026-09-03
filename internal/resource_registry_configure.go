package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRegistryConfigure() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRegistryConfigureCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: resourceRegistryConfigureCreate,
		// Portainer has no endpoint to clear a registry configuration; removing
		// the resource stops managing it and leaves the registry as configured.
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"registry_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the registry to configure.",
			},
			"authentication": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether the registry requires authentication. When false, Portainer ignores `username` and `password`.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username used to authenticate against the registry. Required when `authentication` is true.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password or token paired with `username`. Portainer never returns it, so it cannot be checked for drift.",
			},
			"region": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "AWS region of an ECR registry. Ignored by other registry types.",
			},
			"tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether Portainer contacts the registry over TLS.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the registry's TLS certificate.",
			},
			"tls_ca_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded CA certificate used to verify the registry's certificate. Stored in state as a sensitive value.",
			},
			"tls_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded client certificate presented to the registry. Stored in state as a sensitive value.",
			},
			"tls_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded client private key paired with `tls_cert`. Stored in state as a sensitive value.",
			},
		},
	}
}

func resourceRegistryConfigureCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)

	// registries.registryConfigurePayload uses PascalCase field names. The three
	// TLS files are []byte on the Portainer side, which encoding/json represents
	// as base64 — Go's marshaller does that conversion for a []byte value, so
	// the PEM text is passed through as-is here.
	payload := map[string]interface{}{
		"Authentication": d.Get("authentication").(bool),
		"TLS":            d.Get("tls").(bool),
		"TLSSkipVerify":  d.Get("tls_skip_verify").(bool),
	}
	for field, key := range map[string]string{
		"username": "Username", "password": "Password", "region": "Region",
	} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			payload[key] = v.(string)
		}
	}
	for field, key := range map[string]string{
		"tls_ca_cert": "TLSCACertFile", "tls_cert": "TLSCertFile", "tls_key": "TLSKeyFile",
	} {
		if v, ok := d.GetOk(field); ok && v.(string) != "" {
			payload[key] = []byte(v.(string))
		}
	}

	url := fmt.Sprintf("%s/registries/%d/configure", client.Endpoint, registryID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to configure registry %d: %w", registryID, err))
	}

	d.SetId(fmt.Sprintf("%d/configure", registryID))
	return nil
}
