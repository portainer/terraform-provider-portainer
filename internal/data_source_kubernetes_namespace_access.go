package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// namespaceAccessEntrySchema is one user or team with access to a namespace.
func namespaceAccessEntrySchema(idField, principal string) *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			idField: {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer " + principal + ".",
			},
			"role_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer role the " + principal + " holds.",
			},
			"namespaces": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Namespaces the role applies to, for a namespace-scoped role.",
			},
		},
	}
}

// dataSourceKubernetesNamespaceAccess reads who has access to a namespace,
// the read side of the portainer_kubernetes_namespace_access resource.
func dataSourceKubernetesNamespaceAccess() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesNamespaceAccessRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment owning the namespace.",
			},
			"namespace_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Kubernetes namespace whose access is read.",
			},
			// Computed attributes
			"user_access": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        namespaceAccessEntrySchema("user_id", "user"),
				Description: "Users with access to the namespace, ordered by user ID.",
			},
			"team_access": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        namespaceAccessEntrySchema("team_id", "team"),
				Description: "Teams with access to the namespace, ordered by team ID.",
			},
		},
	}
}

type namespaceAccessPolicy struct {
	RoleID     int      `json:"RoleId"`
	Namespaces []string `json:"Namespaces"`
}

// flattenNamespaceAccess turns an access policy map keyed by ID into a list
// ordered by ID, so the result does not shuffle between reads.
func flattenNamespaceAccess(policies map[string]namespaceAccessPolicy, idField string) []interface{} {
	ids := make([]int, 0, len(policies))
	byID := make(map[int]namespaceAccessPolicy, len(policies))
	for key, p := range policies {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		byID[id] = p
	}
	sort.Ints(ids)
	out := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]interface{}{
			idField:      id,
			"role_id":    byID[id].RoleID,
			"namespaces": byID[id].Namespaces,
		})
	}
	return out
}

func dataSourceKubernetesNamespaceAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace_id").(string)

	var access struct {
		UserAccessPolicies map[string]namespaceAccessPolicy `json:"UserAccessPolicies"`
		TeamAccessPolicies map[string]namespaceAccessPolicy `json:"TeamAccessPolicies"`
	}
	endpoint := fmt.Sprintf("%s/endpoints/%d/namespaces/%s/access", client.Endpoint, endpointID, url.PathEscape(namespace))
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &access); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_namespace_access", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to read the access of namespace %s: %w", namespace, err))
	}

	if err := setFields(d, map[string]interface{}{
		"user_access": flattenNamespaceAccess(access.UserAccessPolicies, "user_id"),
		"team_access": flattenNamespaceAccess(access.TeamAccessPolicies, "team_id"),
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d:%s", endpointID, namespace))
	return nil
}
