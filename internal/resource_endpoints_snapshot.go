package internal

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointsSnapshot() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointsSnapshotCreate,
		Read:   resourceEndpointsSnapshotRead,
		Delete: resourceEndpointsSnapshotDelete,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "ID of the endpoint to snapshot. If omitted, all endpoints will be snapshotted.",
			},
		},
	}
}

func resourceEndpointsSnapshotCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	var req *http.Request
	var err error

	if v, ok := d.GetOk("endpoint_id"); ok {
		id := v.(int)
		req, err = http.NewRequest("POST", fmt.Sprintf("%s/endpoints/%d/snapshot", client.Endpoint, id), nil)
		if err != nil {
			return err
		}
		d.SetId(strconv.Itoa(id))
	} else {
		req, err = http.NewRequest("POST", fmt.Sprintf("%s/endpoints/snapshot", client.Endpoint), nil)
		if err != nil {
			return err
		}
		d.SetId("all")
	}

	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("failed to snapshot endpoint(s): HTTP %d", resp.StatusCode)
	}

	return nil
}

func resourceEndpointsSnapshotRead(d *schema.ResourceData, meta interface{}) error {
	// No meaningful read state; this is a one-time action resource
	return nil
}

func resourceEndpointsSnapshotDelete(d *schema.ResourceData, meta interface{}) error {
	// Nothing to delete in Portainer; just remove from state
	d.SetId("")
	return nil
}
