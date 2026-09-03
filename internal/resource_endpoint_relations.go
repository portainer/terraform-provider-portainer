package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointRelations() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointRelationsApply,
		ReadContext:   schema.NoopContext,
		UpdateContext: resourceEndpointRelationsApply,
		// Portainer has no endpoint to clear relations; removing the resource
		// stops managing them and leaves the environments as they are.
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"relation": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				Description: "Relations to apply, one block per environment. Portainer applies them in a single transaction, so either all of them land or none do.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_id": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Identifier of the environment the relations apply to.",
						},
						"edge_group_ids": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Edge group identifiers the environment belongs to. An empty list leaves the environment's edge groups untouched.",
						},
						"tag_ids": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Tag identifiers assigned to the environment. An empty list leaves the environment's tags untouched.",
						},
						"group_id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Environment group the environment is moved to. Zero, the default, leaves its group untouched — Portainer only applies a non-zero value.",
						},
					},
				},
			},
		},
	}
}

func resourceEndpointRelationsApply(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Relations is keyed by environment ID; JSON object keys are strings, so the
	// numeric IDs are formatted as such.
	relations := map[string]interface{}{}
	for _, raw := range d.Get("relation").([]interface{}) {
		r := raw.(map[string]interface{})
		endpointID := r["endpoint_id"].(int)
		relations[strconv.Itoa(endpointID)] = map[string]interface{}{
			"EdgeGroups": toIntSlice(r["edge_group_ids"].([]interface{})),
			"Tags":       toIntSlice(r["tag_ids"].([]interface{})),
			"Group":      r["group_id"].(int),
		}
	}

	payload := map[string]interface{}{"Relations": relations}
	if err := doJSON(ctx, client, http.MethodPut, client.Endpoint+"/endpoints/relations", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update environment relations: %w", err))
	}

	if d.Id() == "" {
		d.SetId("endpoint-relations-" + strconv.FormatInt(makeTimestamp(), 10))
	}
	return nil
}
