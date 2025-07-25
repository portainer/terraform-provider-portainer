package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type CloudCredentialPayload struct {
	Provider    string                 `json:"provider"`
	Name        string                 `json:"name"`
	Credentials map[string]interface{} `json:"credentials"`
}

func resourceCloudCredentials() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudCredentialsCreate,
		Update: resourceCloudCredentialsUpdate,
		Delete: resourceCloudCredentialsDelete,
		Read:   resourceCloudCredentialsRead,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"cloud_provider": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Cloud provider name (e.g., aws, gcp, digitalocean)",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Human-readable name of the credentials",
			},
			"credentials": {
				Type:        schema.TypeMap,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "JSON-encoded credentials for the provider",
			},
		},
	}
}

func resourceCloudCredentialsCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := CloudCredentialPayload{
		Provider:    d.Get("cloud_provider").(string),
		Name:        d.Get("name").(string),
		Credentials: mapStringInterface(d.Get("credentials").(map[string]interface{})),
	}

	var result struct {
		ID int `json:"id"`
	}

	resp, err := client.DoRequest(http.MethodPost, "/cloud/credentials", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create cloud credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to create cloud credential: HTTP %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func resourceCloudCredentialsDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	path := fmt.Sprintf("/cloud/credentials/%s", d.Id())
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete cloud credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete cloud credential: HTTP %d", resp.StatusCode)
	}

	d.SetId("")
	return nil
}

func resourceCloudCredentialsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	path := fmt.Sprintf("/cloud/credentials/%s", id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read cloud credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to read cloud credential: HTTP %d", resp.StatusCode)
	}

	var result struct {
		ID          int                    `json:"id"`
		Name        string                 `json:"name"`
		Provider    string                 `json:"provider"`
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	d.Set("name", result.Name)
	d.Set("cloud_provider", result.Provider)
	d.Set("credentials", result.Credentials)

	return nil
}

func resourceCloudCredentialsUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	credentialsJSON, err := json.Marshal(mapStringInterface(d.Get("credentials").(map[string]interface{})))
	if err != nil {
		return fmt.Errorf("failed to encode credentials to JSON: %w", err)
	}

	form := map[string]string{
		"provider":    d.Get("cloud_provider").(string),
		"name":        d.Get("name").(string),
		"credentials": string(credentialsJSON),
	}

	resp, err := client.DoRequest(http.MethodPut, fmt.Sprintf("/cloud/credentials/%s", id), form, nil)
	if err != nil {
		return fmt.Errorf("failed to update cloud credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to update cloud credential: HTTP %d", resp.StatusCode)
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
