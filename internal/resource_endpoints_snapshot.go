package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointsSnapshot() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointsSnapshotCreate,
		ReadContext:   resourceEndpointsSnapshotRead,
		DeleteContext: resourceEndpointsSnapshotDelete,
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

func resourceEndpointsSnapshotCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var req *http.Request
	var err error

	if v, ok := d.GetOk("endpoint_id"); ok {
		id := v.(int)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/endpoints/%d/snapshot", client.Endpoint, id), nil)
		if err != nil {
			return diag.FromErr(err)
		}
		d.SetId(strconv.Itoa(id))
	} else {
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/endpoints/snapshot", client.Endpoint), nil)
		if err != nil {
			return diag.FromErr(err)
		}
		d.SetId("all")
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return diag.FromErr(fmt.Errorf("failed to snapshot endpoint(s): HTTP %d", resp.StatusCode))
	}

	return nil
}

func resourceEndpointsSnapshotRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// No meaningful read state; this is a one-time action resource
	return nil
}

func resourceEndpointsSnapshotDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Nothing to delete in Portainer; just remove from state
	d.SetId("")
	return nil
}
