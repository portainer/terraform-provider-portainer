package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesCRD() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceKubernetesCRDRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of a specific CRD to retrieve. If not set, all CRDs are listed.",
			},
			// Computed attributes
			"crds": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "CRD name.",
						},
						"group": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "CRD API group.",
						},
						"scope": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "CRD scope (Namespaced or Cluster).",
						},
						"creation_date": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Creation timestamp.",
						},
						"release_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Helm release name (if applicable).",
						},
						"release_namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Helm release namespace (if applicable).",
						},
						"release_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Helm release version (if applicable).",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesCRDRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	crdName, nameSet := d.GetOk("name")

	if nameSet {
		// Get a specific CRD
		path := fmt.Sprintf("/kubernetes/%d/customresourcedefinitions/%s", envID, crdName.(string))
		resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to get CRD: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("failed to get CRD: HTTP %d", resp.StatusCode)
		}

		var crd struct {
			Name             string `json:"name"`
			Group            string `json:"group"`
			Scope            string `json:"scope"`
			CreationDate     string `json:"creationDate"`
			ReleaseName      string `json:"releaseName"`
			ReleaseNamespace string `json:"releaseNamespace"`
			ReleaseVersion   string `json:"releaseVersion"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&crd); err != nil {
			return fmt.Errorf("failed to decode CRD response: %w", err)
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
		_ = d.Set("crds", crds)
		d.SetId(fmt.Sprintf("%d/%s", envID, crd.Name))
	} else {
		// List all CRDs
		path := fmt.Sprintf("/kubernetes/%d/customresourcedefinitions", envID)
		resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list CRDs: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("failed to list CRDs: HTTP %d", resp.StatusCode)
		}

		var result []struct {
			Name             string `json:"name"`
			Group            string `json:"group"`
			Scope            string `json:"scope"`
			CreationDate     string `json:"creationDate"`
			ReleaseName      string `json:"releaseName"`
			ReleaseNamespace string `json:"releaseNamespace"`
			ReleaseVersion   string `json:"releaseVersion"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode CRDs response: %w", err)
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
		_ = d.Set("crds", crds)
		d.SetId(strconv.FormatInt(time.Now().Unix(), 10) + "/" + strconv.Itoa(envID))
	}

	return nil
}
