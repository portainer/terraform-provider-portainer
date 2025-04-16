package internal

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointAssociationCreate,
		Read:   resourceEndpointAssociationRead,   // no-op
		Delete: resourceEndpointAssociationDelete, // no-op (optional)

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceEndpointAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	url := fmt.Sprintf("%s/endpoints/%d/association", client.Endpoint, endpointID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("failed to de-associate endpoint %d, status code: %d", endpointID, resp.StatusCode)
	}

	d.SetId(strconv.Itoa(endpointID))
	return nil
}

func resourceEndpointAssociationRead(d *schema.ResourceData, meta interface{}) error {
	// This resource is write-only and has no read functionality
	return nil
}

func resourceEndpointAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	// Optionally: remove from state only
	d.SetId("")
	return nil
}
