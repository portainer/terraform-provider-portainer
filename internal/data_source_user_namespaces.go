package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Portainer reports authorizations as an open-ended map of authorization name
// to a boolean, and Portainer adds names to it between releases. Both data
// sources below therefore surface the sorted list of granted names, which is
// what a configuration can act on, alongside the raw response for anything
// else.

// grantedAuthorizations picks out the authorization names that are switched
// on, sorted so the output is stable between two identical plans.
func grantedAuthorizations(authorizations map[string]bool) []string {
	granted := make([]string, 0, len(authorizations))
	for name, allowed := range authorizations {
		if allowed {
			granted = append(granted, name)
		}
	}
	sort.Strings(granted)
	return granted
}

func dataSourceUserNamespaces() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceUserNamespacesRead,

		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the user whose namespace authorizations are read.",
			},
			// Computed attributes
			"namespaces": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The user's namespace authorizations, one entry per environment and namespace.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Identifier of the environment the namespace belongs to.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the namespace.",
						},
						"authorizations": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Authorizations the user holds in that namespace, sorted. An empty list means the namespace is visible but nothing is permitted in it.",
						},
					},
				},
			},
			"raw": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The whole response as Portainer returns it, encoded as JSON. Portainer adds authorization names between releases, so this is what to read when a name is not in the flattened list yet.",
			},
		},
	}
}

func dataSourceUserNamespacesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	// The response is a map of environment identifier to a map of namespace to
	// its authorizations.
	var mapping map[string]map[string]map[string]bool
	namespacesURL := fmt.Sprintf("%s/users/%d/namespaces", client.Endpoint, userID)
	body, err := apiGETRaw(ctx, client, namespacesURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the namespace authorizations of user %d: %w", userID, err))
	}
	if err := json.Unmarshal(body, &mapping); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse the namespace authorizations of user %d: %w", userID, err))
	}

	// Both levels are sorted so two identical plans produce identical output;
	// Go iterates a map in a random order.
	endpointIDs := make([]string, 0, len(mapping))
	for endpointID := range mapping {
		endpointIDs = append(endpointIDs, endpointID)
	}
	sort.Strings(endpointIDs)

	entries := make([]interface{}, 0)
	for _, endpointID := range endpointIDs {
		namespaces := mapping[endpointID]
		names := make([]string, 0, len(namespaces))
		for name := range namespaces {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			entries = append(entries, map[string]interface{}{
				"endpoint_id":    endpointID,
				"namespace":      name,
				"authorizations": grantedAuthorizations(namespaces[name]),
			})
		}
	}

	if err := setFields(d, map[string]interface{}{
		"namespaces": entries,
		"raw":        string(body),
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-user-namespaces-%d", userID))
	return nil
}

func dataSourceCurrentUserAuthorizations() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCurrentUserAuthorizationsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the environment to report the current user's authorizations on.",
			},
			// Computed attributes
			"authorizations": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Authorizations the token's user holds on the environment, sorted.",
			},
			"namespace_authorizations": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Per-namespace authorizations. Empty for administrators, for non-Kubernetes environments, and when the user has cluster-wide access.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the namespace.",
						},
						"authorizations": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Authorizations the user holds in that namespace, sorted.",
						},
					},
				},
			},
		},
	}
}

func dataSourceCurrentUserAuthorizationsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var environment struct {
		Authorizations map[string]bool `json:"Authorizations"`
	}
	authURL := fmt.Sprintf("%s/users/me/auth/%d", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, authURL, nil, &environment); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the current user's authorizations on environment %d: %w", endpointID, err))
	}

	var perNamespace struct {
		NamespaceAuthorizations map[string]map[string]bool `json:"namespaceAuthorizations"`
	}
	namespacesURL := fmt.Sprintf("%s/users/me/auth/%d/namespaces", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, namespacesURL, nil, &perNamespace); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the current user's namespace authorizations on environment %d: %w", endpointID, err))
	}

	names := make([]string, 0, len(perNamespace.NamespaceAuthorizations))
	for name := range perNamespace.NamespaceAuthorizations {
		names = append(names, name)
	}
	sort.Strings(names)

	namespaces := make([]interface{}, 0, len(names))
	for _, name := range names {
		namespaces = append(namespaces, map[string]interface{}{
			"namespace":      name,
			"authorizations": grantedAuthorizations(perNamespace.NamespaceAuthorizations[name]),
		})
	}

	if err := setFields(d, map[string]interface{}{
		"authorizations":           grantedAuthorizations(environment.Authorizations),
		"namespace_authorizations": namespaces,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-current-user-authorizations-%d", endpointID))
	return nil
}
