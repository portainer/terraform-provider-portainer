package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/webhooks"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

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

	params := webhooks.NewPostWebhooksParams()
	params.Body = &models.WebhooksWebhookCreatePayload{
		EndpointID:  int64(d.Get("endpoint_id").(int)),
		RegistryID:  int64(d.Get("registry_id").(int)),
		ResourceID:  d.Get("resource_id").(string),
		WebhookType: int64(d.Get("webhook_type").(int)),
	}

	resp, err := client.Client.Webhooks.PostWebhooks(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	d.Set("token", resp.Payload.Token)
	return nil
}

func resourceWebhookRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceWebhookUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if d.HasChange("registry_id") {
		params := webhooks.NewPutWebhooksIDParams()
		params.ID = id
		params.Body = &models.WebhooksWebhookUpdatePayload{
			RegistryID: int64(d.Get("registry_id").(int)),
		}

		_, err := client.Client.Webhooks.PutWebhooksID(params, client.AuthInfo)
		if err != nil {
			return fmt.Errorf("failed to update webhook: %w", err)
		}
	}

	return resourceWebhookRead(d, meta)
}

func resourceWebhookDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := webhooks.NewDeleteWebhooksIDParams()
	params.ID = id

	_, err := client.Client.Webhooks.DeleteWebhooksID(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	d.SetId("")
	return nil
}
