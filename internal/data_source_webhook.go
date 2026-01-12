package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
	endpointID := d.Get("endpoint_id").(int)

	resp, err := client.DoRequest("GET", "/webhooks", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list webhooks, status %d: %s", resp.StatusCode, string(data))
	}

	var webhooks []struct {
		ID         int    `json:"Id"`
		EndpointID int    `json:"EndpointId"`
		ResourceID string `json:"ResourceId"`
		Token      string `json:"Token"`
		Type       int    `json:"Type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&webhooks); err != nil {
		return fmt.Errorf("failed to decode webhook list: %w", err)
	}

	for _, w := range webhooks {
		if w.ResourceID == resourceID && w.EndpointID == endpointID {
			d.SetId(strconv.Itoa(w.ID))
			d.Set("webhook_type", w.Type)
			d.Set("token", w.Token)
			return nil
		}
	}

	return fmt.Errorf("webhook for resource %s in endpoint %d not found", resourceID, endpointID)
}
