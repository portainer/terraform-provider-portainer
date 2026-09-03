package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceStackDeleteByName() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceStackDeleteByNameCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Kubernetes stack to remove. Every stack with this name in the target environment is removed.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the environment the stack was deployed to.",
			},
			"external": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether the stack was created outside Portainer. Set it to true to remove a stack Portainer only discovered, rather than one it deployed.",
			},
		},
	}
}

func resourceStackDeleteByNameCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	name := d.Get("name").(string)
	endpointID := d.Get("endpoint_id").(int)

	q := url.Values{}
	q.Set("endpointId", fmt.Sprint(endpointID))
	if d.Get("external").(bool) {
		q.Set("external", "true")
	}

	target := fmt.Sprintf("%s/stacks/name/%s?%s", client.Endpoint, url.PathEscape(name), q.Encode())
	if err := doJSON(ctx, client, http.MethodDelete, target, nil, nil); err != nil {
		// A stack that is already gone is the state this resource asks for.
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to remove Kubernetes stack %q from environment %d: %w", name, endpointID, err))
		}
	}

	d.SetId(fmt.Sprintf("%d/%s/deleted/%d", endpointID, name, makeTimestamp()))
	return nil
}
