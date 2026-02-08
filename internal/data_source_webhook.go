package internal

import (
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
				Type:     schema.TypeString,
				Required: true,
			},
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"webhook_type": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"token": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceWebhookRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	resourceID := d.Get("resource_id").(string)
	endpointID := int64(d.Get("endpoint_id").(int))

	params := webhooks.NewGetWebhooksParams()
	resp, err := client.Client.Webhooks.GetWebhooks(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
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
