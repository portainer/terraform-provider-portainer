package internal

import (
	"context"
	"fmt"
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

	url := fmt.Sprintf("%s/endpoints/%d/association", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to de-associate endpoint %d: %w", endpointID, err))
	}

	d.SetId(strconv.Itoa(endpointID))
	return nil
}

func resourceEndpointAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Id()

	// Verify the endpoint still exists via GET /endpoints/{id}
	url := fmt.Sprintf("%s/endpoints/%s", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			// Endpoint no longer exists, remove from state
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read endpoint %s: %w", endpointID, err))
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
