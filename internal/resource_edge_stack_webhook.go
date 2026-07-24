package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerEdgeStackWebhook() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerEdgeStackWebhookCreate,
		ReadContext:   resourcePortainerEdgeStackWebhookRead,
		DeleteContext: resourcePortainerEdgeStackWebhookDelete,
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

func resourcePortainerEdgeStackWebhookCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	webhookID := d.Get("webhook_id").(string)

	url := fmt.Sprintf("%s/edge_stacks/webhooks/%s", client.Endpoint, webhookID)
	if err := doJSON(ctx, client, http.MethodPost, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to trigger webhook: %w", err))
	}

	d.SetId(webhookID)

	return nil
}

func resourcePortainerEdgeStackWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourcePortainerEdgeStackWebhookDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}
