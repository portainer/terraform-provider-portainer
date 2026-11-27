package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesIngressClassesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_ingress_classes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			// Computed attributes
			"ingress_classes": k8sDSObjectList("Ingress classes available in the cluster.",
				k8sDSField{"name", types.StringType, "Name of the ingress class, as referenced by an Ingress `ingressClassName`."},
				k8sDSField{"controller", types.StringType, "Controller implementing this ingress class (for example `k8s.io/ingress-nginx`)."},
				k8sDSField{"is_default", types.BoolType, "Whether this class is the cluster default, applied to Ingresses that name no class."},
				k8sDSField{"annotations", k8sDSStringMap, "Annotations set on the ingress class."},
			),
			"default_ingress_class": k8sDSComputedString("Name of the cluster's default ingress class, empty when no class is marked as default."),
		},
	}, nil, kubernetesIngressClassesRead)
}

func kubernetesIngressClassesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)

	// K8sIngressClass is serialized with PascalCase json tags, unlike most of
	// the Portainer API.
	var result []struct {
		Name        string            `json:"Name"`
		Controller  string            `json:"Controller"`
		IsDefault   bool              `json:"IsDefault"`
		Annotations map[string]string `json:"Annotations"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/ingressclasses", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return fmt.Errorf("failed to list Kubernetes ingress classes: %w", err)
	}

	classes := make([]map[string]interface{}, len(result))
	defaultClass := ""
	for i, c := range result {
		annotations := c.Annotations
		if annotations == nil {
			annotations = map[string]string{}
		}
		classes[i] = map[string]interface{}{
			"name":        c.Name,
			"controller":  c.Controller,
			"is_default":  c.IsDefault,
			"annotations": annotations,
		}
		if c.IsDefault && defaultClass == "" {
			defaultClass = c.Name
		}
	}

	if err := d.SetFields(map[string]interface{}{
		"ingress_classes":       classes,
		"default_ingress_class": defaultClass,
	}); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/ingressclasses")
	return nil
}
