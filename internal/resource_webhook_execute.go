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
				ConflictsWith: []string{"stack_id"},
				Description:   "Webhook token for service restart webhook",
			},
			"stack_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"token"},
				Description:   "Stack ID for triggering stack git update",
			},
		},
	}
}

func resourceWebhookExecuteCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	var url string
	if token, ok := d.GetOk("token"); ok {
		url = fmt.Sprintf("%s/webhooks/%s", client.Endpoint, token)
		d.SetId(token.(string))
	} else if stackID, ok := d.GetOk("stack_id"); ok {
		url = fmt.Sprintf("%s/stacks/webhooks/%s", client.Endpoint, stackID)
		d.SetId(stackID.(string))
	} else {
		return fmt.Errorf("either 'token' or 'stack_id' must be set")
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
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
