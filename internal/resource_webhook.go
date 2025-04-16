package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type WebhookPayload struct {
	EndpointID  int    `json:"endpointID"`
	RegistryID  int    `json:"registryID,omitempty"`
	ResourceID  string `json:"resourceID"`
	WebhookType int    `json:"webhookType"`
}

type WebhookResponse struct {
	ID         int    `json:"Id"`
	EndpointID int    `json:"EndpointId"`
	RegistryID int    `json:"RegistryId"`
	ResourceID string `json:"ResourceId"`
	Token      string `json:"Token"`
	Type       int    `json:"Type"`
}

func resourceWebhook() *schema.Resource {
	return &schema.Resource{
		Create: resourceWebhookCreate,
		Read:   resourceWebhookRead,
		Delete: resourceWebhookDelete,
		Update: resourceWebhookUpdate,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"registry_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"resource_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"webhook_type": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"token": {
				Type:     schema.TypeString,
				Computed: true,
				ForceNew: true,
			},
		},
	}
}

func resourceWebhookCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := WebhookPayload{
		EndpointID:  d.Get("endpoint_id").(int),
		RegistryID:  d.Get("registry_id").(int),
		ResourceID:  d.Get("resource_id").(string),
		WebhookType: d.Get("webhook_type").(int),
	}

	resp, err := client.DoRequest("POST", "/webhooks", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create webhook: %s", string(body))
	}

	var result WebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	d.Set("token", result.Token)
	return nil
}

func resourceWebhookRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceWebhookUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	webhookID := d.Id()

	if d.HasChange("registry_id") {
		payload := map[string]interface{}{
			"registryID": d.Get("registry_id").(int),
		}

		resp, err := client.DoRequest("PUT", fmt.Sprintf("/webhooks/%s", webhookID), nil, payload)
		if err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update webhook: %s", string(body))
		}
	}

	return resourceWebhookRead(d, meta)
}

func resourceWebhookDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	webhookID := d.Id()

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/webhooks/%s", webhookID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete webhook: %s", string(body))
	}

	d.SetId("")
	return nil
}
