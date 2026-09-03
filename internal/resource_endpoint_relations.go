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
							Description: "Edge group identifiers the environment belongs to, replacing whatever it had. Omit the attribute to leave its edge groups alone; there is no way to clear them here, because Portainer cannot tell an intentional \"none\" from \"do not touch\".",
						},
						"tag_ids": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Tag identifiers assigned to the environment, replacing whatever it had. Omit the attribute to leave its tags alone; use `portainer_environment` to clear them.",
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
	//
	// Each field is omitted when it means "leave this alone", because that is
	// exactly how Portainer reads the payload: updateRelations acts on Tags and
	// EdgeGroups only when they are non-nil, and on Group only when it is
	// non-zero. Sending an empty array instead of omitting the key is therefore
	// not a no-op — it tells Portainer to CLEAR that environment's tags or edge
	// groups.
	relations := map[string]interface{}{}
	for _, raw := range d.Get("relation").([]interface{}) {
		r := raw.(map[string]interface{})
		relation := map[string]interface{}{}

		// The SDK fills an omitted nested list with an empty slice rather than
		// nil, so an empty list is the only signal that the operator left the
		// attribute out.
		if ids, ok := r["edge_group_ids"].([]interface{}); ok && len(ids) > 0 {
			relation["EdgeGroups"] = toIntSlice(ids)
		}
		if ids, ok := r["tag_ids"].([]interface{}); ok && len(ids) > 0 {
			relation["Tags"] = toIntSlice(ids)
		}
		if groupID, ok := r["group_id"].(int); ok && groupID != 0 {
			relation["Group"] = groupID
		}

		endpointID := r["endpoint_id"].(int)
		relations[strconv.Itoa(endpointID)] = relation
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
