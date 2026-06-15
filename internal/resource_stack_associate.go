package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceStackAssociate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceStackAssociateCreate,
		ReadContext:   resourceStackAssociateRead,
		DeleteContext: resourceStackAssociateDelete,

		Schema: map[string]*schema.Schema{
			"stack_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the orphaned Portainer stack to associate with an endpoint.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer endpoint that should adopt the orphaned stack.",
			},
			"swarm_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Swarm cluster identifier (`Cluster.ID`) of the target endpoint when associating a Swarm stack.",
			},
			"orphaned_running": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether the orphaned stack is already running on the endpoint; controls Portainer's adoption behaviour.",
			},
		},
	}
}

func resourceStackAssociateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return diag.FromErr(err)
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

	if resp.StatusCode != http.StatusOK {
		return diag.FromErr(fmt.Errorf("failed to associate stack %d to endpoint %d: status code %d", stackID, endpointID, resp.StatusCode))
	}

	d.SetId(fmt.Sprintf("%d", stackID))
	return nil
}

func resourceStackAssociateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceStackAssociateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
