package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// namespaceAccessEntryList is the computed list of users or teams with access
// to a namespace.
func namespaceAccessEntryList(desc, idField, principal string) schema.ListAttribute {
	return k8sDSObjectList(desc,
		k8sDSField{idField, types.Int64Type, "Identifier of the Portainer " + principal + "."},
		k8sDSField{"role_id", types.Int64Type, "Identifier of the Portainer role the " + principal + " holds."},
		k8sDSField{"namespaces", k8sDSStringList, "Namespaces the role applies to, for a namespace-scoped role."},
	)
}

// newKubernetesNamespaceAccessDataSource reads who has access to a namespace,
// the read side of the portainer_kubernetes_namespace_access resource.
func newKubernetesNamespaceAccessDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_namespace_access", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id":  k8sDSRequiredInt("Identifier of the Portainer Kubernetes environment owning the namespace."),
			"namespace_id": k8sDSRequiredString("Name of the Kubernetes namespace whose access is read."),
			// Computed attributes
			"user_access": namespaceAccessEntryList("Users with access to the namespace, ordered by user ID.", "user_id", "user"),
			"team_access": namespaceAccessEntryList("Teams with access to the namespace, ordered by team ID.", "team_id", "team"),
		},
	}, nil, kubernetesNamespaceAccessRead)
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

func kubernetesNamespaceAccessRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace_id").(string)

	var access struct {
		UserAccessPolicies map[string]namespaceAccessPolicy `json:"UserAccessPolicies"`
		TeamAccessPolicies map[string]namespaceAccessPolicy `json:"TeamAccessPolicies"`
	}
	endpoint := fmt.Sprintf("%s/endpoints/%d/namespaces/%s/access", client.Endpoint, endpointID, url.PathEscape(namespace))
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &access); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_namespace_access", requiresBE30)
		return fmt.Errorf("failed to read the access of namespace %s: %w", namespace, err)
	}

	if err := d.SetFields(map[string]interface{}{
		"user_access": flattenNamespaceAccess(access.UserAccessPolicies, "user_id"),
		"team_access": flattenNamespaceAccess(access.TeamAccessPolicies, "team_id"),
	}); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d:%s", endpointID, namespace))
	return nil
}
