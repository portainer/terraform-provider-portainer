package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type CloudCredentialPayload struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Credentials map[string]interface{} `json:"credentials"`
}

func resourceCloudCredentials() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCloudCredentialsCreate,
		UpdateContext: resourceCloudCredentialsUpdate,
		DeleteContext: resourceCloudCredentialsDelete,
		ReadContext:   resourceCloudCredentialsRead,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"cloud_provider": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Cloud provider name (e.g., aws, gcp, digitalocean)",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Human-readable name of the credentials",
				ValidateFunc: validation.NoZeroValues,
			},
			"credentials": {
				Type:        schema.TypeMap,
				Required:    true,
				Sensitive:   true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "JSON-encoded credentials for the provider",
			},
		},
	}
}

func resourceCloudCredentialsCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := CloudCredentialPayload{
		Provider:    d.Get("cloud_provider").(string),
		Name:        d.Get("name").(string),
		Credentials: mapStringInterface(d.Get("credentials").(map[string]interface{})),
	}

	var result struct {
		ID int `json:"id"`
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/cloud/credentials", payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create cloud credential: %w", err))
	}

	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func resourceCloudCredentialsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/cloud/credentials/%s", client.Endpoint, d.Id()), nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete cloud credential: %w", err))
	}

	d.SetId("")
	return nil
}

func resourceCloudCredentialsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	var result struct {
		ID          int                    `json:"id"`
		Name        string                 `json:"name"`
		Provider    string                 `json:"provider"`
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/cloud/credentials/%s", client.Endpoint, id), nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read cloud credential: %w", err))
	}

	if err := d.Set("name", result.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("cloud_provider", result.Provider); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("credentials", result.Credentials); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceCloudCredentialsUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	credentialsJSON, err := json.Marshal(mapStringInterface(d.Get("credentials").(map[string]interface{})))
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to encode credentials to JSON: %w", err))
	}

	form := map[string]string{
		"provider":    d.Get("cloud_provider").(string),
		"name":        d.Get("name").(string),
		"credentials": string(credentialsJSON),
	}

	resp, err := client.DoRequest(http.MethodPut, fmt.Sprintf("/cloud/credentials/%s", id), form, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update cloud credential: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return diag.FromErr(fmt.Errorf("failed to update cloud credential: HTTP %d", resp.StatusCode))
	}

	return nil
}

func mapStringInterface(input map[string]interface{}) map[string]interface{} {
	output := make(map[string]interface{})
	for k, v := range input {
		output[k] = v
	}
	return output
}
