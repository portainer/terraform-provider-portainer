package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerStackWebhook() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerStackWebhookCreate,
		ReadContext:   resourcePortainerStackWebhookRead,
		DeleteContext: resourcePortainerStackWebhookDelete,
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

func resourcePortainerStackWebhookCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	webhookID := d.Get("webhook_id").(string)

	url := fmt.Sprintf("%s/stacks/webhooks/%s", client.Endpoint, webhookID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to build webhook trigger request: %w", err))
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to trigger webhook: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to trigger webhook, status %d: %s", resp.StatusCode, string(body)))
	}

	d.SetId(webhookID)

	return nil
}

func resourcePortainerStackWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourcePortainerStackWebhookDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}
