package internal

import (
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerEdgeStackWebhook() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerEdgeStackWebhookCreate,
		Read:   resourcePortainerEdgeStackWebhookRead,
		Delete: resourcePortainerEdgeStackWebhookDelete,
		Schema: map[string]*schema.Schema{
			"webhook_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "UUID of the webhook to trigger",
				ForceNew:    true,
			},
		},
	}
}

func resourcePortainerEdgeStackWebhookCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	webhookID := d.Get("webhook_id").(string)

	url := fmt.Sprintf("%s/edge_stacks/webhooks/%s", client.Endpoint, webhookID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to build webhook trigger request: %w", err)
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
		return fmt.Errorf("failed to trigger webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger webhook, status %d: %s", resp.StatusCode, string(body))
	}

	d.SetId(webhookID)

	return nil
}

func resourcePortainerEdgeStackWebhookRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourcePortainerEdgeStackWebhookDelete(d *schema.ResourceData, meta interface{}) error {
	return nil
}
