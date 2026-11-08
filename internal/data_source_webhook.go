package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceWebhook() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceWebhookRead,

		Schema: map[string]*schema.Schema{
			"resource_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the resource (e.g. service) the Portainer webhook is attached to. Combined with endpoint_id to find the webhook.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer environment (endpoint) where the webhook is registered.",
			},
			"webhook_type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Type of the Portainer webhook (e.g. service webhook, container webhook).",
			},
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Token used to invoke the Portainer webhook URL.",
			},
		},
	}
}

func dataSourceWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	resourceID := d.Get("resource_id").(string)
	endpointID := d.Get("endpoint_id").(int)

	webhooks, err := listWebhooks(ctx, client, endpointID, resourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, w := range webhooks {
		if w.ResourceID == resourceID && w.EndpointID == endpointID {
			d.SetId(strconv.Itoa(w.ID))
			if err := d.Set("webhook_type", w.Type); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("token", w.Token); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("webhook for resource %s in endpoint %d not found", resourceID, endpointID))
}
