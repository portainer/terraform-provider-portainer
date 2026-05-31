package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/webhooks"
)

func dataSourceWebhook() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceWebhookRead,

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

func dataSourceWebhookRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	resourceID := d.Get("resource_id").(string)
	endpointID := int64(d.Get("endpoint_id").(int))

	ctx, errBody := withErrorCapture(context.Background())
	params := webhooks.NewGetWebhooksParams()
	params.SetContext(ctx)
	resp, err := client.Client.Webhooks.GetWebhooks(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", decorateSDKError(err, errBody))
	}

	for _, w := range resp.Payload {
		if w.ResourceID == resourceID && w.EndpointID == endpointID {
			d.SetId(strconv.FormatInt(w.ID, 10))
			d.Set("webhook_type", int(w.Type))
			d.Set("token", w.Token)
			return nil
		}
	}

	return fmt.Errorf("webhook for resource %s in endpoint %d not found", resourceID, endpointID)
}
