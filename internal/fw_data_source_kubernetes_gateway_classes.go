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

func newKubernetesGatewayClassesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_gateway_classes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			// Computed attributes
			"gateway_classes": k8sDSObjectList("Gateway API GatewayClasses available in the cluster. Empty when no Gateway API implementation is installed.",
				k8sDSField{"name", types.StringType, "Name of the gateway class, as referenced by a Gateway's `class_name`."},
				k8sDSField{"controller_name", types.StringType, "Controller implementing this gateway class, for example `gateway.envoyproxy.io/gatewayclass-controller`."},
				k8sDSField{"uid", types.StringType, "Kubernetes UID of the gateway class."},
				k8sDSField{"labels", k8sDSStringMap, "Labels set on the gateway class."},
				k8sDSField{"annotations", k8sDSStringMap, "Annotations set on the gateway class."},
			),
		},
	}, nil, kubernetesGatewayClassesRead)
}

func kubernetesGatewayClassesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)

	// K8sGatewayClass is serialized with PascalCase json tags, like the other
	// Portainer-native Kubernetes models.
	var result []struct {
		Name           string            `json:"Name"`
		ControllerName string            `json:"ControllerName"`
		UID            string            `json:"UID"`
		Labels         map[string]string `json:"Labels"`
		Annotations    map[string]string `json:"Annotations"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/gatewayclasses", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_gateway_classes", requiresBE30)
		return fmt.Errorf("failed to list Kubernetes gateway classes: %w", err)
	}

	classes := make([]map[string]interface{}, len(result))
	for i, c := range result {
		classes[i] = map[string]interface{}{
			"name":            c.Name,
			"controller_name": c.ControllerName,
			"uid":             c.UID,
			"labels":          stringMap(c.Labels),
			"annotations":     stringMap(c.Annotations),
		}
	}

	if err := d.Set("gateway_classes", classes); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/gatewayclasses")
	return nil
}
