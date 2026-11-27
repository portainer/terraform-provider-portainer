package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesNamespaceResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesNamespaceResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesNamespaceResource)(nil)
)

func newKubernetesNamespaceResource() resource.Resource {
	return &kubernetesNamespaceResource{}
}

type kubernetesNamespaceResource struct{ k8sfwBase }

func (r *kubernetesNamespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_namespace"
}

func (r *kubernetesNamespaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the namespace, `<environment_id>:<name>`.",
			},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the namespace is created.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Kubernetes namespace (metadata.name).",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"owner": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString(""),
				Description:   "Owner label assigned to the namespace in Portainer.",
				PlanModifiers: []planmodifier.String{k8sfwKeepNull{}},
			},
			"annotations": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Map of annotations applied to the Kubernetes namespace.",
			},
			// Kubernetes returns quantities in canonical form: "1000m" comes
			// back as "1", which is the same limit. Read keeps the configured
			// form of a quantity that equals the one reported.
			"resource_quota": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Resource quota limits applied to the namespace (e.g. cpu, memory limits and requests).",
			},
			"resource_quotas": schema.ListAttribute{
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"name": types.StringType,
					"hard": types.MapType{ElemType: types.StringType},
					"used": types.MapType{ElemType: types.StringType},
				}},
				Computed: true,
				Description: "Portainer 3.0+: every Kubernetes ResourceQuota in the namespace as reported by Portainer (`ResourceQuotas`), with hard limits and current usage. " +
					"Empty on releases before 3.0.",
			},
			"effective_ceiling": schema.ListAttribute{
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"resource": types.StringType,
					"hard":     types.StringType,
					"used":     types.StringType,
					"bound_by": types.StringType,
				}},
				Computed: true,
				Description: "Portainer 3.0+: the effective limit per resource across all quotas of the namespace (`EffectiveCeiling`), i.e. the tightest hard limit for cpu/memory, sorted by resource. " +
					"Empty on releases before 3.0.",
			},
			"storage_quotas": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Storage request limit per storage class, as a Kubernetes quantity (e.g. `{ standard = \"10Gi\" }`). Business Edition only. " +
					"Left out, it is not sent; a storage class removed from the map has its quota disabled.",
			},
		},
		Blocks: map[string]schema.Block{
			"load_balancer_quota": schema.ListNestedBlock{
				Description: "Limit of LoadBalancer services in the namespace. Business Edition only. Left out, it is not sent; removing the block disables the quota.",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
						Description: "Whether the LoadBalancer quota is enforced. Defaults to `true`.",
					},
					"limit": schema.Int64Attribute{
						Required:    true,
						Description: "Maximum number of LoadBalancer services in the namespace.",
					},
				}},
			},
		},
	}
}

// Portainer enforces the Business Edition quotas through the namespace's
// ResourceQuota, under the standard Kubernetes resource names.
const (
	namespaceLoadBalancerHardKey   = "services.loadbalancers"
	namespaceStorageClassHardKeyFn = "%s.storageclass.storage.k8s.io/requests.storage"
)

// namespaceLoadBalancerQuota is the LoadBalancerQuota to send: the block,
// a disabled quota when an update removes it, nil when it is not managed.
func namespaceLoadBalancerQuota(data, prior map[string]interface{}) map[string]interface{} {
	if l := k8sfwList(data, "load_balancer_quota"); len(l) > 0 {
		item, _ := l[0].(map[string]interface{})
		enabled, _ := item["enabled"].(bool)
		limit, _ := item["limit"].(int)
		return map[string]interface{}{"enabled": enabled, "limit": limit}
	}
	if len(k8sfwList(prior, "load_balancer_quota")) > 0 {
		return map[string]interface{}{"enabled": false}
	}
	return nil
}

// namespaceStorageQuotas is the StorageQuotas to send: the configured
// classes, and a disabled quota for each class an update removes.
func namespaceStorageQuotas(data, prior map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for class := range k8sfwMap(prior, "storage_quotas") {
		out[class] = map[string]interface{}{"enabled": false}
	}
	for class, limit := range k8sfwStringMap(data["storage_quotas"]) {
		out[class] = map[string]interface{}{"enabled": true, "limit": limit}
	}
	return out
}

// namespaceQuotasFromHard reads the managed Business Edition quotas back
// from the quota's spec.hard. Only what the state manages is refreshed, and
// a quantity equal to the authored one ("1024Mi", "1Gi") keeps its form.
func namespaceQuotasFromHard(data map[string]interface{}, hard map[string]string) {
	if l := k8sfwList(data, "load_balancer_quota"); len(l) > 0 {
		if item, ok := l[0].(map[string]interface{}); ok && item["enabled"] == true {
			if n, err := strconv.Atoi(hard[namespaceLoadBalancerHardKey]); err == nil {
				item["limit"] = n
			}
		}
	}
	storage := k8sfwMap(data, "storage_quotas")
	for class, authored := range storage {
		if v, ok := hard[fmt.Sprintf(namespaceStorageClassHardKeyFn, class)]; ok && !k8sScalarEqual(authored, v) {
			storage[class] = v
		}
	}
	if len(storage) > 0 {
		data["storage_quotas"] = storage
	}
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
			"hard": hard,
			"used": q.Status.Used,
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

// namespaceResourceQuota builds the ResourceQuota payload. Without
// resource_quota the quota is disabled: an enabled quota with empty values
// made Portainer reject the namespace ("Error parsing memory quota value").
func namespaceResourceQuota(data map[string]interface{}, licensed bool) map[string]interface{} {
	quota := k8sfwStringMap(data["resource_quota"])
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

// namespaceQuotaKeepAuthored keeps the value in state of each quota whose
// quantity equals the one reported ("1000m" and "1"), which the SDKv2
// version suppressed in the plan instead.
func namespaceQuotaKeepAuthored(quota map[string]string, authored map[string]interface{}) map[string]string {
	for k, v := range quota {
		if old, ok := authored[k].(string); ok && old != "" && v != "" && k8sScalarEqual(old, v) {
			quota[k] = old
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

func namespaceIDParts(id string) (int, string, error) {
	idParts := strings.SplitN(id, ":", 2)
	if len(idParts) != 2 {
		return 0, "", fmt.Errorf("invalid ID format, expected 'envID:name': %s", id)
	}
	envID, err := strconv.Atoi(idParts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid environment ID in resource ID %q: %w", id, err)
	}
	return envID, idParts[1], nil
}

// read refreshes data from the namespace, reporting false when it is gone.
func (r *kubernetesNamespaceResource) read(ctx context.Context, data map[string]interface{}) (bool, error) {
	id, _ := data["id"].(string)
	envID, name, err := namespaceIDParts(id)
	if err != nil {
		return false, err
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s?withResourceQuota=true", r.client.Endpoint, envID, name)

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
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &ns); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read namespace %q: %w", name, err)
	}

	data["environment_id"] = envID
	data["name"] = ns.Name
	// NamespaceOwner is a Portainer-level label the API does not reliably persist back —
	// GET returns "" even when the namespace was created with an owner. Only overwrite
	// state when the API returns something, so we don't generate permanent drift.
	if ns.NamespaceOwner != "" {
		data["owner"] = ns.NamespaceOwner
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
	// returns an empty spec.hard; skip it in that case so the authored quota stays in
	// state and does not generate permanent drift on over-commit environments.
	authored := k8sfwMap(data, "resource_quota")
	quota := map[string]string{}
	if ns.ResourceQuota != nil {
		quota = namespaceQuotaFromHard(ns.ResourceQuota.Spec.Hard, authored)
		namespaceQuotasFromHard(data, ns.ResourceQuota.Spec.Hard)
	}
	if len(quota) > 0 {
		data["resource_quota"] = namespaceQuotaKeepAuthored(quota, authored)
	}

	data["resource_quotas"] = flattenNamespaceQuotas(ns.ResourceQuotas)
	data["effective_ceiling"] = flattenEffectiveCeiling(ns.EffectiveCeiling)
	return true, nil
}

// write sends the namespace; prior is the state of an update (nil on
// create), to disable the Business Edition quotas the update removes.
func (r *kubernetesNamespaceResource) write(ctx context.Context, data, prior map[string]interface{}, method, url string, diags *diag.Diagnostics) {
	licensed, err := hasLicense(ctx, r.client)
	if err != nil {
		diags.AddError("Failed to check the Portainer license", err.Error())
		return
	}

	body := map[string]interface{}{
		"Name":          data["name"],
		"Owner":         data["owner"],
		"Annotations":   k8sfwStringMap(data["annotations"]),
		"ResourceQuota": namespaceResourceQuota(data, licensed),
	}
	if lb := namespaceLoadBalancerQuota(data, prior); lb != nil {
		body["LoadBalancerQuota"] = lb
	}
	if storage := namespaceStorageQuotas(data, prior); len(storage) > 0 {
		body["StorageQuotas"] = storage
	}
	if err := doJSON(ctx, r.client, method, url, body, nil); err != nil {
		verb := "create"
		if method == http.MethodPut {
			verb = "update"
		}
		diags.AddError("Failed to "+verb+" the namespace", fmt.Errorf("failed to %s namespace: %w", verb, err).Error())
	}
}

func (r *kubernetesNamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := data["environment_id"].(int)
	r.write(ctx, data, nil, http.MethodPost, fmt.Sprintf("%s/kubernetes/%d/namespaces", r.client.Endpoint, id), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data["id"] = fmt.Sprintf("%s:%s", strconv.Itoa(id), data["name"])
	r.readApplied(ctx, data, plan, &resp.State, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// readApplied refreshes data after an apply and stores it.
func (r *kubernetesNamespaceResource) readApplied(ctx context.Context, data map[string]interface{}, plan types.Object, state *tfsdk.State, diags *diag.Diagnostics) {
	found, err := r.read(ctx, data)
	if err != nil {
		diags.AddError("Failed to read the namespace", err.Error())
		return
	}
	if !found {
		diags.AddError("Failed to read the namespace", fmt.Sprintf("namespace %q not found after the apply", data["name"]))
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), state, data, plan, diags)
}

func (r *kubernetesNamespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the namespace", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesNamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	prior, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	idParts := strings.SplitN(id.ValueString(), ":", 2)
	if len(idParts) != 2 {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("invalid ID format, expected 'envID:name': %s", id.ValueString()))
		return
	}
	envID, _ := strconv.Atoi(idParts[0])
	oldName := idParts[1]
	newName, _ := data["name"].(string)

	r.write(ctx, data, prior, http.MethodPut, fmt.Sprintf("%s/kubernetes/%d/namespaces/%s", r.client.Endpoint, envID, oldName), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data["id"] = fmt.Sprintf("%d:%s", envID, newName)
	r.readApplied(ctx, data, plan, &resp.State, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesNamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	idParts := strings.SplitN(id.ValueString(), ":", 2)
	if len(idParts) != 2 {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("invalid ID format, expected 'envID:name': %s", id.ValueString()))
		return
	}
	envID, _ := strconv.Atoi(idParts[0])
	name := idParts[1]

	// Portainer takes a list of namespace names (since 2.27 at least); the
	// object this used to send was rejected with 400 on every destroy.
	body := []string{name}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodDelete, url, body, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete the namespace", fmt.Errorf("failed to delete namespace: %w", err).Error())
	}
}

// ImportState takes "<environment_id>:<name>".
func (r *kubernetesNamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	k8sfwImportDefaults(ctx, k8sfwSchemaOf(ctx, r), &resp.State, &resp.Diagnostics)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
