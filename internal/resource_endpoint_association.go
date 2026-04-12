package internal

import (
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointAssociationCreate,
		Read:   resourceEndpointAssociationRead,
		Delete: resourceEndpointAssociationDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Environment (Endpoint) identifier to de-associate",
			},
		},
	}
}

func resourceEndpointAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/endpoints/%d/association", endpointID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to de-associate endpoint %d: %w", endpointID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to de-associate endpoint %d (status %d): %s", endpointID, resp.StatusCode, string(data))
	}

	d.SetId(strconv.Itoa(endpointID))
	return nil
}

func resourceEndpointAssociationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Id()

	// Verify the endpoint still exists via GET /endpoints/{id}
	resp, err := client.DoRequest("GET", fmt.Sprintf("/endpoints/%s", endpointID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read endpoint %s: %w", endpointID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Endpoint no longer exists, remove from state
		d.SetId("")
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read endpoint %s (status %d): %s", endpointID, resp.StatusCode, string(data))
	}

	// Endpoint exists; keep in state
	id, _ := strconv.Atoi(endpointID)
	d.Set("endpoint_id", id)
	return nil
}

func resourceEndpointAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	// The API only supports de-association (which is the Create action).
	// There is no re-association API, so Delete simply removes from state.
	d.SetId("")
	return nil
}
