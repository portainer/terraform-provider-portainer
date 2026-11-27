package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The thirteen resources below each manage one Kubernetes object described
// by a raw manifest: POST the manifest to the collection, GET the object for
// drift (k8sReadManifestDrift), merge-patch it on change (k8sUpdateManifest)
// and DELETE it. They differ only in the API path, how the object is
// addressed (namespace, volume type) and their texts, so one implementation,
// k8sManifestResource, serves them all, parameterised by a k8sManifestKind.

// k8sManifestScope is how an object kind is addressed, which decides the
// attributes and the import ID format.
type k8sManifestScope int

const (
	// k8sNamespaced: ID endpointID:namespace:name, namespace required.
	k8sNamespaced k8sManifestScope = iota
	// k8sClusterScoped: ID endpointID:name, no namespace attribute.
	k8sClusterScoped
	// k8sVolumeScoped: ID endpointID:namespace:type:name, the URL follows type.
	k8sVolumeScoped
)

// k8sDefaultOpTimeout is SDKv2's default for an operation without a declared
// timeout: it ran every CRUD function under a context with this deadline.
const k8sDefaultOpTimeout = 20 * time.Minute

type k8sManifestKind struct {
	typeName string // after "portainer_"
	scope    k8sManifestScope

	// apiPrefix is the group/version path ("apis/batch/v1", "api/v1") and
	// plural the resource ("cronjobs"); unused for k8sVolumeScoped.
	apiPrefix string
	plural    string

	// label names the object in read/update errors: "<label> <name>".
	label string
	// createNoun and deleteNoun keep the error texts of each SDKv2 resource:
	// "failed to create <createNoun>: ...", "failed to delete <deleteNoun>: ...".
	createNoun string
	deleteNoun string

	endpointDescription  string
	namespaceDescription string
	manifestDescription  string
	manifestSensitive    bool

	// withTimeouts adds the timeouts block with these defaults.
	withTimeouts                                bool
	createTimeout, updateTimeout, deleteTimeout time.Duration
}

var (
	k8sKindApplication = k8sManifestKind{
		typeName: "kubernetes_application", scope: k8sNamespaced,
		apiPrefix: "apis/apps/v1", plural: "deployments", label: "deployment",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes endpoint where the application is deployed.",
		namespaceDescription: "Kubernetes namespace in which the application manifest is applied.",
		manifestDescription:  "YAML or JSON manifest describing the Kubernetes application to deploy.",
		withTimeouts:         true,
		createTimeout:        10 * time.Minute, updateTimeout: 10 * time.Minute, deleteTimeout: 5 * time.Minute,
	}
	k8sKindCronJob = k8sManifestKind{
		typeName: "kubernetes_cronjob", scope: k8sNamespaced,
		apiPrefix: "apis/batch/v1", plural: "cronjobs", label: "cronjob",
		createNoun: "Kubernetes CronJob", deleteNoun: "CronJob",
		endpointDescription:  "Identifier of the Portainer Kubernetes endpoint where the CronJob is deployed.",
		namespaceDescription: "Kubernetes namespace in which the CronJob manifest is applied.",
		manifestDescription:  "YAML or JSON manifest describing the Kubernetes CronJob to deploy.",
	}
	k8sKindJob = k8sManifestKind{
		typeName: "kubernetes_job", scope: k8sNamespaced,
		apiPrefix: "apis/batch/v1", plural: "jobs", label: "job",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes endpoint where the Job is deployed.",
		namespaceDescription: "Kubernetes namespace in which the Job manifest is applied.",
		manifestDescription:  "YAML or JSON manifest describing the Kubernetes Job to deploy.",
	}
	k8sKindServiceAccounts = k8sManifestKind{
		typeName: "kubernetes_serviceaccounts", scope: k8sNamespaced,
		apiPrefix: "api/v1", plural: "serviceaccounts", label: "serviceaccount",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes endpoint where the ServiceAccount is created.",
		namespaceDescription: "Kubernetes namespace in which the ServiceAccount manifest is applied.",
		manifestDescription:  "YAML or JSON manifest describing the Kubernetes ServiceAccount to deploy.",
	}
	k8sKindConfigMaps = k8sManifestKind{
		typeName: "kubernetes_configmaps", scope: k8sNamespaced,
		apiPrefix: "api/v1", plural: "configmaps", label: "configmap",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes environment (endpoint) where the ConfigMap is managed. Changing this value forces resource recreation.",
		namespaceDescription: "Kubernetes namespace in which the ConfigMap is created.",
		manifestDescription:  "Raw YAML or JSON manifest defining the Kubernetes ConfigMap.",
	}
	k8sKindSecret = k8sManifestKind{
		typeName: "kubernetes_secret", scope: k8sNamespaced,
		apiPrefix: "api/v1", plural: "secrets", label: "secret",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes environment (endpoint) where the Secret is managed. Changing this value forces resource recreation.",
		namespaceDescription: "Kubernetes namespace in which the Secret is created.",
		manifestDescription:  "Raw YAML or JSON manifest defining the Kubernetes Secret. Contains the secret data, so it is stored in state as a sensitive value and hidden in plans.",
		manifestSensitive:    true,
	}
	k8sKindService = k8sManifestKind{
		typeName: "kubernetes_service", scope: k8sNamespaced,
		apiPrefix: "api/v1", plural: "services", label: "service",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes environment (endpoint) where the Service is managed. Changing this value forces resource recreation.",
		namespaceDescription: "Kubernetes namespace in which the Service is created.",
		manifestDescription:  "Raw YAML or JSON manifest defining the Kubernetes Service.",
	}
	k8sKindRole = k8sManifestKind{
		typeName: "kubernetes_role", scope: k8sNamespaced,
		apiPrefix: "apis/rbac.authorization.k8s.io/v1", plural: "roles", label: "role",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes environment (endpoint) where the Role is managed. Changing this value forces resource recreation.",
		namespaceDescription: "Kubernetes namespace in which the Role is created.",
		manifestDescription:  "Raw YAML or JSON manifest defining the Kubernetes Role.",
	}
	k8sKindRoleBinding = k8sManifestKind{
		typeName: "kubernetes_rolebinding", scope: k8sNamespaced,
		apiPrefix: "apis/rbac.authorization.k8s.io/v1", plural: "rolebindings", label: "rolebinding",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription:  "Identifier of the Portainer Kubernetes environment (endpoint) where the RoleBinding is managed. Changing this value forces resource recreation.",
		namespaceDescription: "Kubernetes namespace in which the RoleBinding is created.",
		manifestDescription:  "Raw YAML or JSON manifest defining the Kubernetes RoleBinding.",
	}
	k8sKindClusterRole = k8sManifestKind{
		typeName: "kubernetes_clusterrole", scope: k8sClusterScoped,
		apiPrefix: "apis/rbac.authorization.k8s.io/v1", plural: "clusterroles", label: "clusterrole",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription: "Identifier of the Portainer Kubernetes environment (endpoint) where the cluster-scoped ClusterRole is managed. Changing this value forces resource recreation.",
		manifestDescription: "Raw YAML or JSON manifest defining the Kubernetes ClusterRole.",
	}
	k8sKindClusterRoleBinding = k8sManifestKind{
		typeName: "kubernetes_clusterrolebinding", scope: k8sClusterScoped,
		apiPrefix: "apis/rbac.authorization.k8s.io/v1", plural: "clusterrolebindings", label: "clusterrolebinding",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription: "Identifier of the Portainer Kubernetes environment (endpoint) where the cluster-scoped ClusterRoleBinding is managed. Changing this value forces resource recreation.",
		manifestDescription: "Raw YAML or JSON manifest defining the Kubernetes ClusterRoleBinding.",
	}
	k8sKindVolume = k8sManifestKind{
		typeName: "kubernetes_volume", scope: k8sVolumeScoped,
		createNoun: "Kubernetes volume", deleteNoun: "volume",
		endpointDescription:  "Identifier of the Portainer Kubernetes endpoint where the volume resource is managed.",
		namespaceDescription: "Kubernetes namespace for namespaced volume resources (required for `persistent-volume-claim`).",
		manifestDescription:  "YAML or JSON manifest describing the Kubernetes volume resource to deploy.",
	}
	k8sKindStorage = k8sManifestKind{
		typeName: "kubernetes_storage", scope: k8sClusterScoped,
		apiPrefix: "apis/storage.k8s.io/v1", plural: "storageclasses", label: "storageclass",
		createNoun: "Kubernetes Job", deleteNoun: "Job",
		endpointDescription: "Identifier of the Portainer Kubernetes endpoint where the storage class manifest is applied.",
		manifestDescription: "YAML or JSON manifest describing the Kubernetes StorageClass or related storage resources.",
	}
)

// k8sVolumeTypes are the values of portainer_kubernetes_volume.type.
var k8sVolumeTypes = []string{"persistent-volume-claim", "persistent-volume", "volume-attachment"}

func newKubernetesApplicationResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindApplication}
}

func newKubernetesCronJobResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindCronJob}
}

func newKubernetesJobResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindJob}
}

func newKubernetesServiceAccountsResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindServiceAccounts}
}

func newKubernetesConfigMapsResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindConfigMaps}
}

func newKubernetesSecretResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindSecret}
}

func newKubernetesServiceResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindService}
}

func newKubernetesRoleResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindRole}
}

func newKubernetesRoleBindingResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindRoleBinding}
}

func newKubernetesClusterRoleResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindClusterRole}
}

func newKubernetesClusterRoleBindingResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindClusterRoleBinding}
}

func newKubernetesVolumeResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindVolume}
}

func newKubernetesStorageResource() resource.Resource {
	return &k8sManifestResource{kind: k8sKindStorage}
}

var (
	_ resource.Resource                = (*k8sManifestResource)(nil)
	_ resource.ResourceWithConfigure   = (*k8sManifestResource)(nil)
	_ resource.ResourceWithImportState = (*k8sManifestResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*k8sManifestResource)(nil)
	_ resource.ResourceWithIdentity    = (*k8sManifestResource)(nil)
)

type k8sManifestResource struct {
	kind   k8sManifestKind
	client *APIClient
}

// k8sManifestValues holds the attributes of one of these resources. The
// attribute set differs per kind, so it is read and written attribute by
// attribute rather than through a tfsdk struct.
type k8sManifestValues struct {
	ID         types.String
	EndpointID types.Int64
	Namespace  types.String // k8sNamespaced, k8sVolumeScoped
	Type       types.String // k8sVolumeScoped
	Manifest   types.String
	Timeouts   timeouts.Value // withTimeouts
}

func (r *k8sManifestResource) hasNamespace() bool { return r.kind.scope != k8sClusterScoped }

// k8sAttributeGetter is a plan, state or config.
type k8sAttributeGetter interface {
	GetAttribute(context.Context, path.Path, interface{}) diag.Diagnostics
}

func (r *k8sManifestResource) get(ctx context.Context, src k8sAttributeGetter, diags *diag.Diagnostics) k8sManifestValues {
	var v k8sManifestValues
	diags.Append(src.GetAttribute(ctx, path.Root("id"), &v.ID)...)
	diags.Append(src.GetAttribute(ctx, path.Root("endpoint_id"), &v.EndpointID)...)
	diags.Append(src.GetAttribute(ctx, path.Root("manifest"), &v.Manifest)...)
	if r.hasNamespace() {
		diags.Append(src.GetAttribute(ctx, path.Root("namespace"), &v.Namespace)...)
	}
	if r.kind.scope == k8sVolumeScoped {
		diags.Append(src.GetAttribute(ctx, path.Root("type"), &v.Type)...)
	}
	if r.kind.withTimeouts {
		diags.Append(src.GetAttribute(ctx, path.Root("timeouts"), &v.Timeouts)...)
	}
	return v
}

func (r *k8sManifestResource) set(ctx context.Context, state *tfsdk.State, v k8sManifestValues, diags *diag.Diagnostics) {
	diags.Append(state.SetAttribute(ctx, path.Root("id"), v.ID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("endpoint_id"), v.EndpointID)...)
	diags.Append(state.SetAttribute(ctx, path.Root("manifest"), v.Manifest)...)
	if r.hasNamespace() {
		diags.Append(state.SetAttribute(ctx, path.Root("namespace"), v.Namespace)...)
	}
	if r.kind.scope == k8sVolumeScoped {
		diags.Append(state.SetAttribute(ctx, path.Root("type"), v.Type)...)
	}
	if r.kind.withTimeouts {
		diags.Append(state.SetAttribute(ctx, path.Root("timeouts"), v.Timeouts)...)
	}
}

func (r *k8sManifestResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind.typeName
	// The identity follows the ID, which an in-place rename or namespace
	// change replaces (see ModifyPlan).
	resp.ResourceBehavior.MutableIdentity = true
}

func (r *k8sManifestResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description:   "Identifier of the resource.",
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"endpoint_id": schema.Int64Attribute{
			Required:      true,
			Description:   r.kind.endpointDescription,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
		},
		"manifest": schema.StringAttribute{
			Required:    true,
			Sensitive:   r.kind.manifestSensitive,
			Description: r.kind.manifestDescription,
		},
	}
	switch r.kind.scope {
	case k8sNamespaced:
		attrs["namespace"] = schema.StringAttribute{
			Required:    true,
			Description: r.kind.namespaceDescription,
		}
	case k8sVolumeScoped:
		attrs["namespace"] = schema.StringAttribute{
			Optional:    true,
			Description: r.kind.namespaceDescription,
		}
		attrs["type"] = schema.StringAttribute{
			Required:    true,
			Description: "Type of Kubernetes volume resource: `persistent-volume-claim`, `persistent-volume`, or `volume-attachment`.",
			Validators:  []validator.String{stringvalidator.OneOf(k8sVolumeTypes...)},
		}
	}
	s := schema.Schema{Attributes: attrs}
	if r.kind.withTimeouts {
		s.Blocks = map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		}
	}
	resp.Schema = s
}

func (r *k8sManifestResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *k8sManifestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "" {
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
		return
	}
	r.importByIdentity(ctx, req, resp)
}

// ModifyPlan plans the new ID of an update that recreates the object - a new
// metadata.name, namespace or volume type - which Update then sets. Any other
// update keeps the ID (UseStateForUnknown).
func (r *k8sManifestResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	plan := r.get(ctx, req.Plan, &resp.Diagnostics)
	state := r.get(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Manifest.IsUnknown() || plan.Namespace.IsUnknown() || plan.Type.IsUnknown() || plan.EndpointID.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
		return
	}
	parsed, err := parseManifest(plan.Manifest.ValueString())
	if err != nil {
		return // Update reports it.
	}
	meta, _ := parsed["metadata"].(map[string]interface{})
	name, _ := meta["name"].(string)
	if name == "" {
		return
	}
	_, _, _, oldName := k8sManifestParseID(r.kind.scope, state.ID.ValueString())
	identityChanged := plan.Namespace.ValueString() != state.Namespace.ValueString() ||
		plan.Type.ValueString() != state.Type.ValueString()
	if name == oldName && !identityChanged {
		return
	}
	newID := r.kind.id(int(plan.EndpointID.ValueInt64()), plan.Namespace.ValueString(), plan.Type.ValueString(), name)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringValue(newID))...)
	r.kind.setIdentity(ctx, newID, resp.Identity, &resp.Diagnostics)
}

// k8sManifestError reports err the way diag.FromErr did: its text is the
// summary.
func k8sManifestError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

// timeout returns the deadline of an operation: the timeouts block where the
// resource has one, else SDKv2's default.
func (r *k8sManifestResource) timeout(ctx context.Context, v k8sManifestValues, op string, diags *diag.Diagnostics) time.Duration {
	if !r.kind.withTimeouts {
		return k8sDefaultOpTimeout
	}
	var (
		d  time.Duration
		dd diag.Diagnostics
	)
	switch op {
	case "create":
		d, dd = v.Timeouts.Create(ctx, r.kind.createTimeout)
	case "update":
		d, dd = v.Timeouts.Update(ctx, r.kind.updateTimeout)
	case "delete":
		d, dd = v.Timeouts.Delete(ctx, r.kind.deleteTimeout)
	}
	diags.Append(dd...)
	return d
}

// k8sManifestParseID splits a resource ID into its parts; a malformed ID gives
// zero values, which Read reports.
func k8sManifestParseID(scope k8sManifestScope, id string) (endpointID int, namespace, volType, name string) {
	switch scope {
	case k8sClusterScoped:
		parts := strings.SplitN(id, ":", 3)
		if len(parts) < 2 {
			return 0, "", "", ""
		}
		_, _ = fmt.Sscanf(parts[0], "%d", &endpointID)
		return endpointID, "", "", parts[1]
	case k8sVolumeScoped:
		parts := strings.SplitN(id, ":", 4)
		if len(parts) != 4 {
			return 0, "", "", ""
		}
		_, _ = fmt.Sscanf(parts[0], "%d", &endpointID)
		return endpointID, parts[1], parts[2], parts[3]
	default:
		parts := strings.SplitN(id, ":", 3)
		if len(parts) != 3 {
			return 0, "", "", ""
		}
		_, _ = fmt.Sscanf(parts[0], "%d", &endpointID)
		return endpointID, parts[1], "", parts[2]
	}
}

// k8sManifestID builds the resource ID of an object.
func (k k8sManifestKind) id(endpointID int, namespace, volType, name string) string {
	switch k.scope {
	case k8sClusterScoped:
		return fmt.Sprintf("%d:%s", endpointID, name)
	case k8sVolumeScoped:
		return fmt.Sprintf("%d:%s:%s:%s", endpointID, namespace, volType, name)
	default:
		return fmt.Sprintf("%d:%s:%s", endpointID, namespace, name)
	}
}

// url is the collection URL (name empty) or the object URL.
func (k k8sManifestKind) url(base string, endpointID int, namespace, volType, name string) (string, error) {
	switch k.scope {
	case k8sVolumeScoped:
		if name == "" {
			return volumeAPIURL(base, endpointID, namespace, volType, false)
		}
		return volumeAPIURL(base, endpointID, namespace, volType, true, name)
	case k8sClusterScoped:
		u := fmt.Sprintf("%s/endpoints/%d/kubernetes/%s/%s", base, endpointID, k.apiPrefix, k.plural)
		if name != "" {
			u += "/" + name
		}
		return u, nil
	default:
		u := fmt.Sprintf("%s/endpoints/%d/kubernetes/%s/namespaces/%s/%s", base, endpointID, k.apiPrefix, namespace, k.plural)
		if name != "" {
			u += "/" + name
		}
		return u, nil
	}
}

// objectLabel names the object in read and update errors.
func (k k8sManifestKind) objectLabel(volType, name string) string {
	if k.scope == k8sVolumeScoped {
		return volType + " " + name
	}
	return k.label + " " + name
}

// invalidIDError is Read's error for an ID it cannot parse.
func (k k8sManifestKind) invalidIDError(id string, endpointID int, namespace, volType, name string) error {
	switch k.scope {
	case k8sClusterScoped:
		if endpointID == 0 || name == "" {
			return fmt.Errorf("invalid ID format, expected 'endpointID:name': %s", id)
		}
	case k8sVolumeScoped:
		if endpointID == 0 || volType == "" || name == "" {
			return fmt.Errorf("invalid ID format, expected 'endpointID:namespace:type:name': %s", id)
		}
	default:
		if endpointID == 0 || namespace == "" || name == "" {
			return fmt.Errorf("invalid ID format, expected 'endpointID:namespace:name': %s", id)
		}
	}
	return nil
}

// createObject POSTs the manifest of v and returns the new resource ID.
func (r *k8sManifestResource) createObject(ctx context.Context, v k8sManifestValues, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpointID := int(v.EndpointID.ValueInt64())
	namespace := v.Namespace.ValueString()
	volType := v.Type.ValueString()

	parsed, err := parseManifest(v.Manifest.ValueString())
	if err != nil {
		return "", fmt.Errorf("manifest must be valid JSON or YAML: %w", err)
	}
	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing metadata in manifest")
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("missing metadata.name in manifest")
	}

	url, err := r.kind.url(r.client.Endpoint, endpointID, namespace, volType, "")
	if err != nil {
		return "", err
	}
	if err := doJSON(ctx, r.client, http.MethodPost, url, parsed, nil); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", r.kind.createNoun, err)
	}
	return r.kind.id(endpointID, namespace, volType, name), nil
}

// deleteObject DELETEs the object of resource ID id; a 404 is success.
func (r *k8sManifestResource) deleteObject(ctx context.Context, id string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpointID, namespace, volType, name := k8sManifestParseID(r.kind.scope, id)
	url, err := r.kind.url(r.client.Endpoint, endpointID, namespace, volType, name)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if err := setAuthHeader(req, r.client); err != nil {
		return err
	}
	resp, err := r.client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", r.kind.deleteNoun, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete %s: %s", r.kind.deleteNoun, string(body))
	}
	return nil
}

func (r *k8sManifestResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan := r.get(ctx, req.Plan, &resp.Diagnostics)
	timeout := r.timeout(ctx, plan, "create", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.createObject(ctx, plan, timeout)
	if err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		return
	}
	plan.ID = types.StringValue(id)
	r.set(ctx, &resp.State, plan, &resp.Diagnostics)
	r.kind.setIdentity(ctx, id, resp.Identity, &resp.Diagnostics)
}

func (r *k8sManifestResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state := r.get(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// First, so state written by v3.0.0 (no identity) gets one even when the
	// object turns out to be gone.
	r.kind.setIdentity(ctx, state.ID.ValueString(), resp.Identity, &resp.Diagnostics)
	ctx, cancel := context.WithTimeout(ctx, k8sDefaultOpTimeout)
	defer cancel()

	id := state.ID.ValueString()
	endpointID, namespace, volType, name := k8sManifestParseID(r.kind.scope, id)
	if err := r.kind.invalidIDError(id, endpointID, namespace, volType, name); err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		return
	}
	url, err := r.kind.url(r.client.Endpoint, endpointID, namespace, volType, name)
	if err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		return
	}

	gone, drifted, err := k8sReadManifestDrift(ctx, r.client, url, r.kind.objectLabel(volType, name), state.Manifest.ValueString())
	if err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		return
	}
	if gone {
		resp.State.RemoveResource(ctx)
		return
	}

	state.EndpointID = types.Int64Value(int64(endpointID))
	switch r.kind.scope {
	case k8sNamespaced:
		state.Namespace = types.StringValue(namespace)
	case k8sVolumeScoped:
		// A cluster-scoped volume has no namespace: an unset one stays null.
		if namespace != "" || !state.Namespace.IsNull() {
			state.Namespace = types.StringValue(namespace)
		}
		state.Type = types.StringValue(volType)
	}
	// manifest only changes in state when the live object drifted — see k8sReadManifestDrift.
	if drifted != "" {
		state.Manifest = types.StringValue(drifted)
	}
	r.set(ctx, &resp.State, state, &resp.Diagnostics)
}

func (r *k8sManifestResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan := r.get(ctx, req.Plan, &resp.Diagnostics)
	state := r.get(ctx, req.State, &resp.Diagnostics)
	updateTimeout := r.timeout(ctx, plan, "update", &resp.Diagnostics)
	createTimeout := r.timeout(ctx, plan, "create", &resp.Diagnostics)
	deleteTimeout := r.timeout(ctx, state, "delete", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	oldID := state.ID.ValueString()
	endpointID, namespace, volType, name := k8sManifestParseID(r.kind.scope, oldID)
	url, err := r.kind.url(r.client.Endpoint, endpointID, namespace, volType, name)
	if err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		return
	}
	identityChanged := plan.Namespace.ValueString() != state.Namespace.ValueString() ||
		plan.Type.ValueString() != state.Type.ValueString()

	newID, deleted := oldID, false
	err = k8sUpdateManifest(ctx, r.client, url, name, r.kind.objectLabel(volType, name),
		state.Manifest.ValueString(), plan.Manifest.ValueString(), identityChanged,
		func() error {
			if err := r.deleteObject(ctx, oldID, deleteTimeout); err != nil {
				return err
			}
			deleted = true
			return nil
		},
		func() error {
			id, err := r.createObject(ctx, plan, createTimeout)
			if err != nil {
				return err
			}
			newID = id
			return nil
		})
	if err != nil {
		k8sManifestError(&resp.Diagnostics, err)
		if deleted && newID == oldID {
			// Deleted but not created again: the object is gone.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.State.Raw = req.State.Raw
		return
	}

	plan.ID = types.StringValue(newID)
	r.set(ctx, &resp.State, plan, &resp.Diagnostics)
	r.kind.setIdentity(ctx, newID, resp.Identity, &resp.Diagnostics)
}

func (r *k8sManifestResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state := r.get(ctx, req.State, &resp.Diagnostics)
	timeout := r.timeout(ctx, state, "delete", &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.deleteObject(ctx, state.ID.ValueString(), timeout); err != nil {
		k8sManifestError(&resp.Diagnostics, err)
	}
}

// volumeAPIURL builds the correct URL for the volume type
func volumeAPIURL(base string, endpointID int, namespace string, volType string, withName bool, name ...string) (string, error) {
	var path string

	switch volType {
	case "persistent-volume-claim":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/namespaces/%s/persistentvolumeclaims/%s", endpointID, namespace, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/namespaces/%s/persistentvolumeclaims", endpointID, namespace)
		}
	case "persistent-volume":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/persistentvolumes/%s", endpointID, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/persistentvolumes", endpointID)
		}
	case "volume-attachment":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/apis/storage.k8s.io/v1/volumeattachments/%s", endpointID, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/apis/storage.k8s.io/v1/volumeattachments", endpointID)
		}
	default:
		return "", fmt.Errorf("unsupported volume type: %s", volType)
	}

	return base + path, nil
}
