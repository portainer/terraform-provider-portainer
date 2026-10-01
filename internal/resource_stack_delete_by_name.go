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
			// Portainer rejects this call with 400 "Invalid query parameter:
			// namespace" whenever the parameter is missing or empty, regardless
			// of `external`, and it checks it before it even resolves the
			// environment. The OpenAPI specification does not document the
			// parameter at all, which is how it came to be left out: the
			// resource could not have worked without it.
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace the stack was deployed into. Portainer requires it, even though its API specification does not list it.",
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
	q.Set("namespace", d.Get("namespace").(string))
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
