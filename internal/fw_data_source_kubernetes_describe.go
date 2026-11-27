package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func newKubernetesDescribeDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_describe", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"kind":           k8sDSRequiredString("Kind of the resource to describe, as `kubectl describe` takes it (for example `pod`, `deployment` or `node`)."),
			"name":           k8sDSRequiredString("Name of the resource to describe."),
			"namespace":      k8sDSOptionalString("Namespace of the resource. Leave unset for a cluster-scoped kind such as `node`."),
			// Computed attributes
			"describe": k8sDSComputedString("Output of `kubectl describe` for the resource, as plain text."),
		},
	}, nil, kubernetesDescribeRead)
}

func kubernetesDescribeRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)

	q := url.Values{}
	q.Set("kind", d.Get("kind").(string))
	q.Set("name", d.Get("name").(string))
	if v, ok := d.GetOk("namespace"); ok {
		q.Set("namespace", v.(string))
	}

	var result struct {
		Describe string `json:"describe"`
	}
	target := fmt.Sprintf("%s/kubernetes/%d/describe?%s", client.Endpoint, envID, q.Encode())
	if err := doJSON(ctx, client, http.MethodGet, target, nil, &result); err != nil {
		return fmt.Errorf("failed to describe %s/%s: %w", d.Get("kind").(string), d.Get("name").(string), err)
	}

	if err := d.Set("describe", result.Describe); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/%s/%s", envID, d.Get("namespace").(string), d.Get("kind").(string), d.Get("name").(string)))
	return nil
}
