package internal

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceStackMigrate() *schema.Resource {
	return &schema.Resource{
		Create: resourceStackMigrateCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"stack_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Stack identifier to migrate.",
			},
			"target_endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Target environment (endpoint) identifier to migrate the stack to.",
			},
			"stack_name": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "New name for the stack after migration. If not set, the original name is kept.",
			},
			"swarm_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Swarm cluster identifier (required when migrating to a Swarm environment).",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional source environment (endpoint) identifier. Required for stacks created before Portainer 1.18.0.",
			},
		},
	}
}

func resourceStackMigrateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	stackID := d.Get("stack_id").(int)
	targetEndpointID := d.Get("target_endpoint_id").(int)

	payload := map[string]interface{}{
		"EndpointID": targetEndpointID,
	}

	if v, ok := d.GetOk("stack_name"); ok {
		payload["Name"] = v.(string)
	}

	if v, ok := d.GetOk("swarm_id"); ok {
		payload["SwarmID"] = v.(string)
	}

	path := fmt.Sprintf("/stacks/%d/migrate", stackID)
	if v, ok := d.GetOk("endpoint_id"); ok {
		path = fmt.Sprintf("%s?endpointId=%d", path, v.(int))
	}

	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to migrate stack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to migrate stack: HTTP %d", resp.StatusCode)
	}

	d.SetId(strconv.Itoa(stackID) + "-" + strconv.FormatInt(time.Now().Unix(), 10))
	return nil
}
