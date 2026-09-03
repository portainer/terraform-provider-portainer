package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Status values reported per document by POST /kubernetes/{id}/manifests/dry_run.
const (
	manifestDryRunStatusPass = "pass"
	manifestDryRunStatusFail = "fail"
)

func dataSourceKubernetesManifestDryRun() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesManifestDryRunRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to validate against.",
			},
			"manifests": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Kubernetes manifests to validate, as YAML or JSON. Each entry may contain multiple documents separated by `---`. At least one non-empty manifest is required.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace applied to documents that do not declare one. Leave unset to validate the manifests exactly as written.",
			},
			"fail_on_invalid": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether a document failing validation makes the data source itself fail. Defaults to false, which reports the outcome in `results` and `passed` so the configuration can decide what to do.",
			},
			// Computed attributes
			"passed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether every submitted document passed server-side validation.",
			},
			"failed_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of documents that failed validation.",
			},
			"results": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Per-document validation outcome returned by Portainer, in submission order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"kind": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes kind of the validated document, empty when the document was rejected before its kind could be read.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the validated resource, empty when the document was rejected before its resource could be named.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace the document was validated against.",
						},
						"document_index": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Zero-based position of the document among all non-empty documents submitted, used to identify a document that could not be named (such as malformed YAML).",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Validation outcome for the document, either `pass` or `fail`.",
						},
						"message": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Validation error reported by the Kubernetes API server, empty when the document passed.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesManifestDryRunRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)

	rawManifests := d.Get("manifests").([]interface{})
	manifests := make([]string, len(rawManifests))
	for i, m := range rawManifests {
		manifests[i] = m.(string)
	}

	payload := map[string]interface{}{"manifests": manifests}
	if ns, ok := d.GetOk("namespace"); ok {
		payload["namespace"] = ns.(string)
	}

	var response struct {
		Results []struct {
			Kind          string `json:"kind"`
			Name          string `json:"name"`
			Namespace     string `json:"namespace"`
			DocumentIndex int    `json:"documentIndex"`
			Status        string `json:"status"`
			Message       string `json:"message"`
		} `json:"results"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/manifests/dry_run", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to dry-run Kubernetes manifests: %w", err))
	}

	results := make([]map[string]interface{}, len(response.Results))
	failures := []string{}
	for i, r := range response.Results {
		results[i] = map[string]interface{}{
			"kind":           r.Kind,
			"name":           r.Name,
			"namespace":      r.Namespace,
			"document_index": r.DocumentIndex,
			"status":         r.Status,
			"message":        r.Message,
		}
		if r.Status != manifestDryRunStatusPass {
			failures = append(failures, fmt.Sprintf("document %d (%s/%s): %s", r.DocumentIndex, r.Kind, r.Name, r.Message))
		}
	}

	if err := setFields(d, map[string]interface{}{
		"results":      results,
		"passed":       len(failures) == 0,
		"failed_count": len(failures),
	}); err != nil {
		return diag.FromErr(err)
	}

	// The dry run itself succeeded even when a document is invalid, so the ID is
	// set before the opt-in failure below: the outcome stays readable in state.
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10) + "/" + strconv.Itoa(envID))

	if len(failures) > 0 && d.Get("fail_on_invalid").(bool) {
		return diag.FromErr(fmt.Errorf("%d of %d Kubernetes manifest document(s) failed validation:\n  %s",
			len(failures), len(response.Results), strings.Join(failures, "\n  ")))
	}

	return nil
}
