package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesNamespace() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesNamespaceCreate,
		ReadContext:   resourceKubernetesNamespaceRead,
		UpdateContext: resourceKubernetesNamespaceUpdate,
		DeleteContext: resourceKubernetesNamespaceDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the namespace is created.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Kubernetes namespace (metadata.name).",
			},
			"owner": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Owner label assigned to the namespace in Portainer.",
			},
			"annotations": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Map of annotations applied to the Kubernetes namespace.",
			},
			"resource_quota": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				// Kubernetes returns quantities in canonical form: "1000m"
				// comes back as "1", which is the same limit.
				DiffSuppressFunc: func(_, old, new string, _ *schema.ResourceData) bool {
					return old != "" && new != "" && k8sScalarEqual(old, new)
				},
				Description: "Resource quota limits applied to the namespace (e.g. cpu, memory limits and requests).",
			},
			"resource_quotas": {
				Type:     schema.TypeList,
				Computed: true,
				Description: "Portainer 3.0+: every Kubernetes ResourceQuota in the namespace as reported by Portainer (`ResourceQuotas`), with hard limits and current usage. " +
					"Empty on releases before 3.0.",
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"name": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Name of the ResourceQuota object.",
					},
					"hard": {
						Type:        schema.TypeMap,
						Computed:    true,
						Elem:        &schema.Schema{Type: schema.TypeString},
						Description: "Hard limits per resource (e.g. `limits.cpu` = `2`, `requests.memory` = `1Gi`): `spec.hard`, or `status.hard` when the spec is empty.",
					},
					"used": {
						Type:        schema.TypeMap,
						Computed:    true,
						Elem:        &schema.Schema{Type: schema.TypeString},
						Description: "Current usage per resource (`status.used`).",
					},
				}},
			},
			"effective_ceiling": {
				Type:     schema.TypeList,
				Computed: true,
				Description: "Portainer 3.0+: the effective limit per resource across all quotas of the namespace (`EffectiveCeiling`), i.e. the tightest hard limit for cpu/memory, sorted by resource. " +
					"Empty on releases before 3.0.",
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"resource": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Resource the ceiling applies to, as keyed by Portainer (e.g. `limits.cpu`, `requests.memory`).",
					},
					"hard": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Effective hard limit (Kubernetes quantity, e.g. `500m`, `2Gi`).",
					},
					"used": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Current usage against the limit (Kubernetes quantity).",
					},
					"bound_by": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "Name of the ResourceQuota that sets this limit.",
					},
				}},
			},
		},
	}
}

func resourceKubernetesNamespaceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	licensed, err := hasLicense(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	annotations := map[string]string{}
	if raw, ok := d.GetOk("annotations"); ok {
		for k, v := range raw.(map[string]interface{}) {
			annotations[k] = v.(string)
		}
	}

	rq := namespaceResourceQuota(d, licensed)

	body := map[string]interface{}{
		"Name":          d.Get("name").(string),
		"Owner":         d.Get("owner").(string),
		"Annotations":   annotations,
		"ResourceQuota": rq,
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, id)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create namespace: %w", err))
	}

	envID := strconv.Itoa(id)
	d.SetId(fmt.Sprintf("%s:%s", envID, d.Get("name").(string)))
	return resourceKubernetesNamespaceRead(ctx, d, meta)
}

func resourceKubernetesNamespaceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, err := strconv.Atoi(idParts[0])
	if err != nil {
		return diag.FromErr(fmt.Errorf("invalid environment ID in resource ID %q: %w", d.Id(), err))
	}
	name := idParts[1]

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s?withResourceQuota=true", client.Endpoint, envID, name)

	var ns struct {
		Name           string `json:"Name"`
		NamespaceOwner string `json:"NamespaceOwner"`
		ResourceQuota  *struct {
			Spec struct {
				Hard map[string]string `json:"hard"`
			} `json:"spec"`
		} `json:"ResourceQuota"`
		// Added in Portainer 3.0 (models.K8sNamespaceResponse); absent before.
		ResourceQuotas   []k8sNamespaceQuota               `json:"ResourceQuotas"`
		EffectiveCeiling map[string]k8sEffectiveQuotaLimit `json:"EffectiveCeiling"`
	}
	// Namespace deleted out-of-band — clear from state so the next plan recreates.
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &ns); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read namespace %q: %w", name, err))
	}

	if err := d.Set("environment_id", envID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", ns.Name); err != nil {
		return diag.FromErr(err)
	}
	// NamespaceOwner is a Portainer-level label the API does not reliably persist back —
	// GET returns "" even when the namespace was created with an owner. Only overwrite
	// state when the API returns something, so we don't generate permanent drift.
	if ns.NamespaceOwner != "" {
		if err := d.Set("owner", ns.NamespaceOwner); err != nil {
			return diag.FromErr(err)
		}
	}

	// annotations intentionally not refreshed — the live namespace object includes
	// system-managed annotations the user never authored (e.g.
	// kubectl.kubernetes.io/last-applied-configuration, controller-injected keys).
	// Writing the full server map back into state would produce a permanent diff
	// against the user's config. The authored annotations stay the source of truth;
	// after `terraform import`, set this field in config to match what's live.

	// Map the K8s ResourceQuota spec.hard keys ("limits.cpu", "requests.memory", …) back
	// to the friendlier schema keys (cpu_limit, memory_limit, cpu_request, memory_request).
	// When EnableResourceOverCommit is true Portainer does not persist quotas and the API
	// returns an empty spec.hard; skip d.Set in that case so the authored quota stays in
	// state and does not generate permanent drift on over-commit environments.
	quota := map[string]string{}
	if ns.ResourceQuota != nil {
		quota = namespaceQuotaFromHard(ns.ResourceQuota.Spec.Hard, d.Get("resource_quota").(map[string]interface{}))
	}
	if len(quota) > 0 {
		if err := d.Set("resource_quota", quota); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("resource_quotas", flattenNamespaceQuotas(ns.ResourceQuotas)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("effective_ceiling", flattenEffectiveCeiling(ns.EffectiveCeiling)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// k8sNamespaceQuota is the part of a v1.ResourceQuota the provider exposes.
type k8sNamespaceQuota struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Hard map[string]string `json:"hard"`
	} `json:"spec"`
	Status struct {
		Hard map[string]string `json:"hard"`
		Used map[string]string `json:"used"`
	} `json:"status"`
}

// k8sEffectiveQuotaLimit is models.K8sEffectiveQuotaLimit (Portainer 3.0).
// Hard and Used are resource.Quantity values, which serialise as strings.
type k8sEffectiveQuotaLimit struct {
	BoundBy string `json:"BoundBy"`
	Hard    string `json:"Hard"`
	Used    string `json:"Used"`
}

func flattenNamespaceQuotas(quotas []k8sNamespaceQuota) []interface{} {
	out := make([]interface{}, 0, len(quotas))
	for _, q := range quotas {
		hard := q.Spec.Hard
		if len(hard) == 0 {
			hard = q.Status.Hard
		}
		out = append(out, map[string]interface{}{
			"name": q.Metadata.Name,
			"hard": stringMapToInterface(hard),
			"used": stringMapToInterface(q.Status.Used),
		})
	}
	return out
}

func flattenEffectiveCeiling(ceiling map[string]k8sEffectiveQuotaLimit) []interface{} {
	keys := make([]string, 0, len(ceiling))
	for k := range ceiling {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		c := ceiling[k]
		out = append(out, map[string]interface{}{
			"resource": k,
			"hard":     c.Hard,
			"used":     c.Used,
			"bound_by": c.BoundBy,
		})
	}
	return out
}

func stringMapToInterface(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func resourceKubernetesNamespaceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	licensed, err := hasLicense(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, _ := strconv.Atoi(idParts[0])
	oldName := idParts[1]
	newName := d.Get("name").(string)

	annotations := map[string]string{}
	if raw, ok := d.GetOk("annotations"); ok {
		for k, v := range raw.(map[string]interface{}) {
			annotations[k] = v.(string)
		}
	}

	rq := namespaceResourceQuota(d, licensed)

	body := map[string]interface{}{
		"Name":          newName,
		"Owner":         d.Get("owner").(string),
		"Annotations":   annotations,
		"ResourceQuota": rq,
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s", client.Endpoint, envID, oldName)
	if err := doJSON(ctx, client, http.MethodPut, url, body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update namespace: %w", err))
	}

	// If name changed, update ID
	if oldName != newName {
		d.SetId(fmt.Sprintf("%d:%s", envID, newName))
	}

	return resourceKubernetesNamespaceRead(ctx, d, meta)
}

func resourceKubernetesNamespaceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, _ := strconv.Atoi(idParts[0])
	name := idParts[1]

	// Portainer takes a list of namespace names (since 2.27 at least); the
	// object this used to send was rejected with 400 on every destroy.
	body := []string{name}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodDelete, url, body, nil); err != nil && !isAPINotFound(err) {
		return diag.FromErr(fmt.Errorf("failed to delete namespace: %w", err))
	}

	d.SetId("")
	return nil
}

// namespaceResourceQuota builds the ResourceQuota payload. Without
// resource_quota the quota is disabled: an enabled quota with empty values
// made Portainer reject the namespace ("Error parsing memory quota value").
func namespaceResourceQuota(d *schema.ResourceData, licensed bool) map[string]interface{} {
	quota := map[string]string{}
	if raw, ok := d.GetOk("resource_quota"); ok {
		for k, v := range raw.(map[string]interface{}) {
			quota[k] = v.(string)
		}
	}
	if len(quota) == 0 {
		return map[string]interface{}{"enabled": false}
	}
	if licensed {
		return map[string]interface{}{
			"enabled":       true,
			"cpuRequest":    quota["cpu_request"],
			"cpuLimit":      quota["cpu_limit"],
			"memoryRequest": quota["memory_request"],
			"memoryLimit":   quota["memory_limit"],
		}
	}
	return map[string]interface{}{
		"enabled": true,
		"cpu":     quota["cpu"],
		"memory":  quota["memory"],
	}
}

// namespaceQuotaFromHard maps the quota's spec.hard back to the keys the
// configuration uses. Without a license Portainer takes one cpu/memory value
// and sets it as both the limit and the request, so those keys come back as
// cpu/memory rather than as the *_limit/*_request pair, which planned a change
// on every run. An import, which has no keys yet, uses cpu/memory when the
// limits equal the requests.
func namespaceQuotaFromHard(hard map[string]string, configured map[string]interface{}) map[string]string {
	_, cpu := configured["cpu"]
	_, memory := configured["memory"]
	combined := cpu || memory
	if len(configured) == 0 {
		combined = hard["limits.cpu"] == hard["requests.cpu"] && hard["limits.memory"] == hard["requests.memory"]
	}

	quota := map[string]string{}
	keys := map[string]string{
		"limits.cpu":      "cpu_limit",
		"limits.memory":   "memory_limit",
		"requests.cpu":    "cpu_request",
		"requests.memory": "memory_request",
	}
	if combined {
		keys = map[string]string{"limits.cpu": "cpu", "limits.memory": "memory"}
	}
	for hardKey, key := range keys {
		if v, ok := hard[hardKey]; ok {
			quota[key] = v
		}
	}
	return quota
}

func hasLicense(ctx context.Context, client *APIClient) (bool, error) {
	url := fmt.Sprintf("%s/licenses", client.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	if err := setAuthHeader(req, client); err != nil {
		return false, err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var licenses []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&licenses); err != nil {
		return false, err
	}

	return len(licenses) > 0, nil
}
