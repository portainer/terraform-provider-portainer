package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Status values reported per document by POST /kubernetes/{id}/manifests/dry_run.
const (
	manifestDryRunStatusPass = "pass"
	manifestDryRunStatusFail = "fail"
)

func newKubernetesManifestDryRunDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_manifest_dry_run", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to validate against."),
			"manifests": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
				Description: "Kubernetes manifests to validate, as YAML or JSON. Each entry may contain multiple documents separated by `---`. At least one non-empty manifest is required.",
			},
			"namespace":       k8sDSOptionalString("Namespace applied to documents that do not declare one. Leave unset to validate the manifests exactly as written."),
			"fail_on_invalid": k8sDSOptionalBoolDefault("Whether a document failing validation makes the data source itself fail. Defaults to false, which reports the outcome in `results` and `passed` so the configuration can decide what to do."),
			// Computed attributes
			"passed":       k8sDSComputedBool("Whether every submitted document passed server-side validation."),
			"failed_count": k8sDSComputedInt("Number of documents that failed validation."),
			"results": k8sDSObjectList("Per-document validation outcome returned by Portainer, in submission order.",
				k8sDSField{"kind", types.StringType, "Kubernetes kind of the validated document, empty when the document was rejected before its kind could be read."},
				k8sDSField{"name", types.StringType, "Name of the validated resource, empty when the document was rejected before its resource could be named."},
				k8sDSField{"namespace", types.StringType, "Namespace the document was validated against."},
				k8sDSField{"document_index", types.Int64Type, "Zero-based position of the document among all non-empty documents submitted, used to identify a document that could not be named (such as malformed YAML)."},
				k8sDSField{"status", types.StringType, "Validation outcome for the document, either `pass` or `fail`."},
				k8sDSField{"message", types.StringType, "Validation error reported by the Kubernetes API server, empty when the document passed."},
			),
		},
	}, map[string]interface{}{"fail_on_invalid": false}, kubernetesManifestDryRunRead)
}

func kubernetesManifestDryRunRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to dry-run Kubernetes manifests: %w", err)
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

	if err := d.SetFields(map[string]interface{}{
		"results":      results,
		"passed":       len(failures) == 0,
		"failed_count": len(failures),
	}); err != nil {
		return err
	}

	// The dry run itself succeeded even when a document is invalid, so the ID is
	// set before the opt-in failure below: the outcome stays readable in state.
	d.SetID(strconv.FormatInt(time.Now().Unix(), 10) + "/" + strconv.Itoa(envID))

	if len(failures) > 0 && d.Get("fail_on_invalid").(bool) {
		return fmt.Errorf("%d of %d Kubernetes manifest document(s) failed validation:\n  %s",
			len(failures), len(response.Results), strings.Join(failures, "\n  "))
	}

	return nil
}
