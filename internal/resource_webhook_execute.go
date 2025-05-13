package internal

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceWebhookExecute() *schema.Resource {
	return &schema.Resource{
		Create: resourceWebhookExecuteCreate,
		Read:   resourceWebhookExecuteRead,
		Delete: resourceWebhookExecuteDelete,
		Schema: map[string]*schema.Schema{
			"token": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"stack_id", "edge_stack_id"},
				Description:   "Webhook token for service restart webhook",
			},
			"stack_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"token", "edge_stack_id"},
				Description:   "Stack ID for triggering stack GitOps update",
			},
			"edge_stack_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"token", "stack_id"},
				Description:   "Edge Stack ID for triggering edge stack GitOps update",
			},
		},
	}
}

func resourceWebhookExecuteCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	var url string
	switch {
	case d.Get("token").(string) != "":
		token := d.Get("token").(string)
		url = fmt.Sprintf("%s/webhooks/%s", client.Endpoint, token)
		d.SetId(token)

	case d.Get("stack_id").(string) != "":
		stackID := d.Get("stack_id").(string)
		url = fmt.Sprintf("%s/stacks/webhooks/%s", client.Endpoint, stackID)
		d.SetId(stackID)

	case d.Get("edge_stack_id").(string) != "":
		edgeStackID := d.Get("edge_stack_id").(string)
		url = fmt.Sprintf("%s/edge_stacks/webhooks/%s", client.Endpoint, edgeStackID)
		d.SetId(edgeStackID)

	default:
		return fmt.Errorf("one of 'token', 'stack_id' or 'edge_stack_id' must be set")
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to execute webhook: HTTP %d", resp.StatusCode)
	}

	return nil
}

func resourceWebhookExecuteRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceWebhookExecuteDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
