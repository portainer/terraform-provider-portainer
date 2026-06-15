package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointAssociationCreate,
		ReadContext:   resourceEndpointAssociationRead,
		DeleteContext: resourceEndpointAssociationDelete,

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

func resourceEndpointAssociationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/endpoints/%d/association", endpointID), nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to de-associate endpoint %d: %w", endpointID, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to de-associate endpoint %d (status %d): %s", endpointID, resp.StatusCode, string(data)))
	}

	d.SetId(strconv.Itoa(endpointID))
	return nil
}

func resourceEndpointAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Id()

	// Verify the endpoint still exists via GET /endpoints/{id}
	resp, err := client.DoRequest("GET", fmt.Sprintf("/endpoints/%s", endpointID), nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read endpoint %s: %w", endpointID, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Endpoint no longer exists, remove from state
		d.SetId("")
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read endpoint %s (status %d): %s", endpointID, resp.StatusCode, string(data)))
	}

	// Endpoint exists; keep in state
	id, _ := strconv.Atoi(endpointID)
	if err := d.Set("endpoint_id", id); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceEndpointAssociationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// The API only supports de-association (which is the Create action).
	// There is no re-association API, so Delete simply removes from state.
	d.SetId("")
	return nil
}
