package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceHelmReleaseHistory() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceHelmReleaseHistoryRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (Endpoint) identifier",
			},
			"release_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Helm release",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes namespace of the release",
			},
			// Computed output
			"revisions": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of release revisions",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"revision": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Revision number",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Status of the revision (e.g. deployed, superseded)",
						},
						"chart": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Chart name and version",
						},
						"app_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Application version",
						},
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Description of the revision",
						},
						"updated": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Timestamp when this revision was last deployed",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the release",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the release",
						},
					},
				},
			},
		},
	}
}

func dataSourceHelmReleaseHistoryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	releaseName := d.Get("release_name").(string)

	path := fmt.Sprintf("/endpoints/%d/kubernetes/helm/%s/history", endpointID, releaseName)

	if v, ok := d.GetOk("namespace"); ok {
		path += "?namespace=" + v.(string)
	}

	resp, err := client.DoRequest("GET", path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get Helm release history: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get Helm release history (status %d): %s", resp.StatusCode, string(data))
	}

	var releases []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return fmt.Errorf("failed to decode Helm release history: %w", err)
	}

	revisions := make([]map[string]interface{}, 0, len(releases))
	for _, r := range releases {
		rev := map[string]interface{}{
			"name":      "",
			"namespace": "",
		}

		if v, ok := r["version"]; ok {
			rev["revision"] = int(v.(float64))
		}
		if v, ok := r["name"]; ok {
			rev["name"] = v.(string)
		}
		if v, ok := r["namespace"]; ok {
			rev["namespace"] = v.(string)
		}
		if v, ok := r["appVersion"]; ok {
			rev["app_version"] = v.(string)
		}

		// Extract info fields
		if info, ok := r["info"]; ok && info != nil {
			infoMap := info.(map[string]interface{})
			if v, ok := infoMap["status"]; ok {
				rev["status"] = v.(string)
			}
			if v, ok := infoMap["description"]; ok {
				rev["description"] = v.(string)
			}
			if v, ok := infoMap["last_deployed"]; ok {
				rev["updated"] = v.(string)
			}
		}

		// Extract chart name
		if chart, ok := r["chart"]; ok && chart != nil {
			chartMap := chart.(map[string]interface{})
			if metadata, ok := chartMap["metadata"]; ok && metadata != nil {
				metaMap := metadata.(map[string]interface{})
				chartName := ""
				chartVersion := ""
				if v, ok := metaMap["name"]; ok {
					chartName = v.(string)
				}
				if v, ok := metaMap["version"]; ok {
					chartVersion = v.(string)
				}
				if chartName != "" && chartVersion != "" {
					rev["chart"] = chartName + "-" + chartVersion
				} else {
					rev["chart"] = chartName
				}
			}
		}

		revisions = append(revisions, rev)
	}

	d.SetId(fmt.Sprintf("helm-history-%d-%s", endpointID, releaseName))
	d.Set("revisions", revisions)

	return nil
}
