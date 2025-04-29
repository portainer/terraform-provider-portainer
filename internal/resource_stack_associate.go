package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceStackAssociate() *schema.Resource {
	return &schema.Resource{
		Create: resourceStackAssociateCreate,
		Read:   resourceStackAssociateRead,
		Delete: resourceStackAssociateDelete,

		Schema: map[string]*schema.Schema{
			"stack_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"swarm_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"orphaned_running": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				ForceNew: true,
			},
		},
	}
}

func resourceStackAssociateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	stackID := d.Get("stack_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	swarmID := d.Get("swarm_id").(string)
	orphanedRunning := d.Get("orphaned_running").(bool)

	apiURL := fmt.Sprintf("%s/stacks/%d/associate", client.Endpoint, stackID)

	params := url.Values{}
	params.Add("endpointId", strconv.Itoa(endpointID))
	params.Add("swarmId", swarmID)
	params.Add("orphanedRunning", strconv.FormatBool(orphanedRunning))

	req, err := http.NewRequest("PUT", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to associate stack %d to endpoint %d: status code %d", stackID, endpointID, resp.StatusCode)
	}

	d.SetId(fmt.Sprintf("%d", stackID))
	return nil
}

func resourceStackAssociateRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceStackAssociateDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
