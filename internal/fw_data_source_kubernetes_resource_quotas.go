package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesResourceQuotasDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_resource_quotas", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSRequiredString("Namespace whose resource quotas are listed."),
			// Computed attributes
			"resource_quotas": k8sDSObjectList("Resource quotas defined in the namespace.",
				k8sDSField{"name", types.StringType, "Name of the resource quota object."},
				k8sDSField{"namespace", types.StringType, "Namespace the quota applies to."},
				k8sDSField{"hard", k8sDSStringMap, "Enforced limit per resource name (for example `limits.cpu` or `requests.memory`), as Kubernetes quantity strings."},
				k8sDSField{"used", k8sDSStringMap, "Current usage per resource name, as Kubernetes quantity strings."},
				k8sDSField{"scopes", k8sDSStringList, "Scopes the quota is restricted to (for example `BestEffort` or `NotTerminating`), empty when the quota applies to every object."},
				k8sDSField{"creation_timestamp", types.StringType, "RFC3339 creation timestamp of the quota object."},
			),
		},
	}, nil, kubernetesResourceQuotasRead)
}

func kubernetesResourceQuotasRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	// The endpoint returns a corev1.ResourceQuotaList verbatim, so quantities
	// arrive as Kubernetes quantity strings ("2", "500m", "1Gi").
	var result struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Hard   map[string]string `json:"hard"`
				Scopes []string          `json:"scopes"`
			} `json:"spec"`
			Status struct {
				Hard map[string]string `json:"hard"`
				Used map[string]string `json:"used"`
			} `json:"status"`
		} `json:"items"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/resource_quotas", client.Endpoint, envID, namespace)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return fmt.Errorf("failed to list Kubernetes resource quotas in namespace %q: %w", namespace, err)
	}

	quotas := make([]map[string]interface{}, len(result.Items))
	for i, q := range result.Items {
		// status.hard is what the quota controller actually enforces; it matches
		// spec.hard once observed, so spec is only the fallback before then.
		hard := q.Status.Hard
		if len(hard) == 0 {
			hard = q.Spec.Hard
		}
		used := q.Status.Used
		if used == nil {
			used = map[string]string{}
		}
		if hard == nil {
			hard = map[string]string{}
		}
		scopes := q.Spec.Scopes
		if scopes == nil {
			scopes = []string{}
		}

		quotas[i] = map[string]interface{}{
			"name":               q.Metadata.Name,
			"namespace":          q.Metadata.Namespace,
			"hard":               hard,
			"used":               used,
			"scopes":             scopes,
			"creation_timestamp": q.Metadata.CreationTimestamp,
		}
	}

	if err := d.Set("resource_quotas", quotas); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/resource_quotas", envID, namespace))
	return nil
}
