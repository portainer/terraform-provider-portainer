package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesCRDDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_crd", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier."),
			"name":           k8sDSOptionalString("Name of a specific CRD to retrieve. If not set, all CRDs are listed."),
			// Computed attributes
			"crds": k8sDSObjectList("List of Kubernetes Custom Resource Definitions returned by the Portainer API.",
				k8sDSField{"name", types.StringType, "CRD name."},
				k8sDSField{"group", types.StringType, "CRD API group."},
				k8sDSField{"scope", types.StringType, "CRD scope (Namespaced or Cluster)."},
				k8sDSField{"creation_date", types.StringType, "Creation timestamp."},
				k8sDSField{"release_name", types.StringType, "Helm release name (if applicable)."},
				k8sDSField{"release_namespace", types.StringType, "Helm release namespace (if applicable)."},
				k8sDSField{"release_version", types.StringType, "Helm release version (if applicable)."},
			),
		},
	}, nil, kubernetesCRDRead)
}

func kubernetesCRDRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	crdName, nameSet := d.GetOk("name")

	if nameSet {
		// Get a specific CRD
		path := fmt.Sprintf("/kubernetes/%d/customresourcedefinitions/%s", envID, crdName.(string))
		var crd struct {
			Name             string `json:"name"`
			Group            string `json:"group"`
			Scope            string `json:"scope"`
			CreationDate     string `json:"creationDate"`
			ReleaseName      string `json:"releaseName"`
			ReleaseNamespace string `json:"releaseNamespace"`
			ReleaseVersion   string `json:"releaseVersion"`
		}
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &crd); err != nil {
			return fmt.Errorf("failed to get CRD: %w", err)
		}

		crds := []map[string]interface{}{
			{
				"name":              crd.Name,
				"group":             crd.Group,
				"scope":             crd.Scope,
				"creation_date":     crd.CreationDate,
				"release_name":      crd.ReleaseName,
				"release_namespace": crd.ReleaseNamespace,
				"release_version":   crd.ReleaseVersion,
			},
		}
		if err := d.Set("crds", crds); err != nil {
			return err
		}
		d.SetID(fmt.Sprintf("%d/%s", envID, crd.Name))
		return nil
	}

	// List all CRDs
	path := fmt.Sprintf("/kubernetes/%d/customresourcedefinitions", envID)
	var result []struct {
		Name             string `json:"name"`
		Group            string `json:"group"`
		Scope            string `json:"scope"`
		CreationDate     string `json:"creationDate"`
		ReleaseName      string `json:"releaseName"`
		ReleaseNamespace string `json:"releaseNamespace"`
		ReleaseVersion   string `json:"releaseVersion"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &result); err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	crds := make([]map[string]interface{}, len(result))
	for i, crd := range result {
		crds[i] = map[string]interface{}{
			"name":              crd.Name,
			"group":             crd.Group,
			"scope":             crd.Scope,
			"creation_date":     crd.CreationDate,
			"release_name":      crd.ReleaseName,
			"release_namespace": crd.ReleaseNamespace,
			"release_version":   crd.ReleaseVersion,
		}
	}
	if err := d.Set("crds", crds); err != nil {
		return err
	}
	d.SetID(strconv.FormatInt(time.Now().Unix(), 10) + "/" + strconv.Itoa(envID))
	return nil
}
