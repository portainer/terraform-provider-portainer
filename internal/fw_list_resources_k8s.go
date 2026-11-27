package internal

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

// Resource identity and list resources (`terraform query`) of the Kubernetes
// manifest resources (fw_resource_kubernetes_manifest.go), the Edge stacks,
// jobs and configurations and the GitOps sources and workflows.
//
// A manifest resource's identity is the parts of its composite ID:
// endpoint_id, namespace (namespaced kinds and volumes), type (volumes) and
// name. The other resources have the { id } identity (fwIDIdentity).

// IdentitySchema of the manifest resources: the parts of the import ID.
func (r *k8sManifestResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	attrs := map[string]identityschema.Attribute{
		"endpoint_id": identityschema.Int64Attribute{
			RequiredForImport: true,
			Description:       "Identifier of the Portainer Kubernetes environment (endpoint) holding the object.",
		},
		"name": identityschema.StringAttribute{
			RequiredForImport: true,
			Description:       "Name of the Kubernetes object (metadata.name).",
		},
	}
	switch r.kind.scope {
	case k8sNamespaced:
		attrs["namespace"] = identityschema.StringAttribute{
			RequiredForImport: true,
			Description:       "Kubernetes namespace of the object.",
		}
	case k8sVolumeScoped:
		// A persistent volume or volume attachment has no namespace.
		attrs["namespace"] = identityschema.StringAttribute{
			OptionalForImport: true,
			Description:       "Kubernetes namespace of a persistent volume claim; unset for the cluster-scoped persistent volumes and volume attachments.",
		}
		attrs["type"] = identityschema.StringAttribute{
			RequiredForImport: true,
			Description:       "Type of the volume object: `persistent-volume-claim`, `persistent-volume` or `volume-attachment`.",
		}
	}
	resp.IdentitySchema = identityschema.Schema{Attributes: attrs}
}

// setIdentity stores the parts of resource ID id as the identity. An ID that
// does not parse leaves it alone (Read reports that ID).
func (k k8sManifestKind) setIdentity(ctx context.Context, id string, identity *tfsdk.ResourceIdentity, diags *diag.Diagnostics) {
	if identity == nil || id == "" {
		return
	}
	endpointID, namespace, volType, name := k8sManifestParseID(k.scope, id)
	if k.invalidIDError(id, endpointID, namespace, volType, name) != nil {
		return
	}
	diags.Append(identity.SetAttribute(ctx, path.Root("endpoint_id"), int64(endpointID))...)
	diags.Append(identity.SetAttribute(ctx, path.Root("name"), name)...)
	switch k.scope {
	case k8sNamespaced:
		diags.Append(identity.SetAttribute(ctx, path.Root("namespace"), namespace)...)
	case k8sVolumeScoped:
		ns := types.StringNull()
		if namespace != "" {
			ns = types.StringValue(namespace)
		}
		diags.Append(identity.SetAttribute(ctx, path.Root("namespace"), ns)...)
		diags.Append(identity.SetAttribute(ctx, path.Root("type"), volType)...)
	}
}

// importByIdentity builds the resource ID from an import block's identity;
// the Read that follows reports an object that does not exist.
func (r *k8sManifestResource) importByIdentity(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var (
		endpointID               types.Int64
		namespace, volType, name types.String
	)
	resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("endpoint_id"), &endpointID)...)
	resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("name"), &name)...)
	if r.hasNamespace() {
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("namespace"), &namespace)...)
	}
	if r.kind.scope == k8sVolumeScoped {
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("type"), &volType)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	id := r.kind.id(int(endpointID.ValueInt64()), namespace.ValueString(), volType.ValueString(), name.ValueString())
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// k8sManifestListResource lists the objects of one manifest resource kind in
// an environment.
type k8sManifestListResource struct {
	kind   k8sManifestKind
	client *APIClient
}

var (
	_ list.ListResource              = (*k8sManifestListResource)(nil)
	_ list.ListResourceWithConfigure = (*k8sManifestListResource)(nil)
)

func newK8sManifestList(kind k8sManifestKind) func() list.ListResource {
	return func() list.ListResource { return &k8sManifestListResource{kind: kind} }
}

func (r *k8sManifestListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind.typeName
}

func (r *k8sManifestListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// listNoun names the objects of a kind in descriptions.
func (k k8sManifestKind) listNoun() string {
	if k.scope == k8sVolumeScoped {
		return "persistent volume claims, persistent volumes and volume attachments"
	}
	return k.plural
}

func (r *k8sManifestListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	attrs := map[string]listschema.Attribute{
		"endpoint_id": listschema.Int64Attribute{
			Required:    true,
			Description: "Identifier of the Portainer Kubernetes environment (endpoint) to list the objects of.",
		},
	}
	switch r.kind.scope {
	case k8sNamespaced:
		attrs["namespace"] = listschema.StringAttribute{
			Optional:    true,
			Description: "Only list the objects of this namespace. Unset lists every namespace.",
		}
	case k8sVolumeScoped:
		attrs["namespace"] = listschema.StringAttribute{
			Optional:    true,
			Description: "Only list the persistent volume claims of this namespace (the cluster-scoped types are then left out). Unset lists every namespace.",
		}
		attrs["type"] = listschema.StringAttribute{
			Optional:    true,
			Description: "Only list this type of volume object: `persistent-volume-claim`, `persistent-volume` or `volume-attachment`. Unset lists all three.",
			Validators:  []validator.String{stringvalidator.OneOf(k8sVolumeTypes...)},
		}
	}
	desc := fmt.Sprintf("Lists the Kubernetes %s of a Portainer environment, through Portainer's Kubernetes API proxy. "+
		"Objects controlled by another object (a Job of a CronJob, for example) are left out. "+
		"With include_resource the manifest is the live object without the fields the API server manages (status, uid, resourceVersion, managedFields, ...).",
		r.kind.listNoun())
	if r.kind.manifestSensitive {
		desc += " Secret data is only part of a result with include_resource, in the sensitive manifest, as the resource stores it."
	}
	resp.Schema = listschema.Schema{Description: desc, Attributes: attrs}
}

// k8sListItem is one object a manifest list found.
type k8sListItem struct {
	id, display, manifest string
}

// k8sManifestListResponse is a Kubernetes list (DeploymentList, ...).
type k8sManifestListResponse struct {
	APIVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Items      []map[string]interface{} `json:"items"`
}

// listURL is the collection to list: every namespace when namespace is "".
func (k k8sManifestKind) listURL(base string, endpointID int, namespace, volType string) (string, error) {
	switch k.scope {
	case k8sVolumeScoped:
		if volType == "persistent-volume-claim" && namespace == "" {
			return fmt.Sprintf("%s/endpoints/%d/kubernetes/api/v1/persistentvolumeclaims", base, endpointID), nil
		}
		return volumeAPIURL(base, endpointID, namespace, volType, false)
	case k8sNamespaced:
		if namespace == "" {
			return fmt.Sprintf("%s/endpoints/%d/kubernetes/%s/%s", base, endpointID, k.apiPrefix, k.plural), nil
		}
	}
	return k.url(base, endpointID, namespace, volType, "")
}

// k8sVolumeListAPIVersion is the apiVersion of the objects of a volume type.
var k8sVolumeListAPIVersion = map[string]string{
	"persistent-volume-claim": "v1",
	"persistent-volume":       "v1",
	"volume-attachment":       "storage.k8s.io/v1",
}

func (r *k8sManifestListResource) items(ctx context.Context, config tfsdk.Config, diags *diag.Diagnostics) []k8sListItem {
	endpointID := int(fwListFilterInt64(ctx, config, "endpoint_id", diags))
	namespace, volFilter := "", ""
	if r.kind.scope != k8sClusterScoped {
		namespace = fwListFilter(ctx, config, "namespace", diags)
	}
	if r.kind.scope == k8sVolumeScoped {
		volFilter = fwListFilter(ctx, config, "type", diags)
	}
	if diags.HasError() {
		return nil
	}

	volTypes := []string{""}
	if r.kind.scope == k8sVolumeScoped {
		volTypes = k8sVolumeTypes
		if volFilter != "" {
			volTypes = []string{volFilter}
		}
	}
	var out []k8sListItem
	for _, volType := range volTypes {
		if r.kind.scope == k8sVolumeScoped && namespace != "" && volType != "persistent-volume-claim" {
			continue // cluster-scoped: no object is in the namespace
		}
		u, err := r.kind.listURL(r.client.Endpoint, endpointID, namespace, volType)
		if err != nil {
			diags.AddError("Failed to list Kubernetes objects", err.Error())
			return nil
		}
		var resp k8sManifestListResponse
		if err := doJSON(ctx, r.client, http.MethodGet, u, nil, &resp); err != nil {
			diags.AddError("Failed to list Kubernetes objects", fmt.Sprintf("failed to list %s of environment %d: %s", r.kind.listNoun(), endpointID, err))
			return nil
		}
		apiVersion := resp.APIVersion
		if apiVersion == "" {
			apiVersion = strings.TrimPrefix(strings.TrimPrefix(r.kind.apiPrefix, "apis/"), "api/")
			if r.kind.scope == k8sVolumeScoped {
				apiVersion = k8sVolumeListAPIVersion[volType]
			}
		}
		kind := strings.TrimSuffix(resp.Kind, "List")
		for _, obj := range resp.Items {
			meta, _ := obj["metadata"].(map[string]interface{})
			name, _ := meta["name"].(string)
			ns, _ := meta["namespace"].(string)
			if name == "" || k8sControlled(meta) {
				continue
			}
			if r.kind.scope == k8sNamespaced && ns == "" {
				continue
			}
			display := name
			if ns != "" && r.kind.scope != k8sClusterScoped {
				display = ns + "/" + name
			}
			if r.kind.scope == k8sVolumeScoped {
				display = volType + " " + display
			}
			manifest, err := k8sListManifest(obj, apiVersion, kind)
			if err != nil {
				diags.AddError("Failed to list Kubernetes objects", fmt.Sprintf("failed to render %s: %s", display, err))
				return nil
			}
			out = append(out, k8sListItem{
				id:       r.kind.id(endpointID, ns, volType, name),
				display:  display,
				manifest: manifest,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// k8sControlled reports an object another object controls (a Job of a
// CronJob, a ReplicaSet of a Deployment): managing it would fight its owner.
func k8sControlled(meta map[string]interface{}) bool {
	refs, _ := meta["ownerReferences"].([]interface{})
	for _, ref := range refs {
		if m, ok := ref.(map[string]interface{}); ok && m["controller"] == true {
			return true
		}
	}
	return false
}

// k8sServerManagedMetadata are the metadata fields the API server sets.
var k8sServerManagedMetadata = []string{
	"uid", "resourceVersion", "generation", "creationTimestamp", "deletionTimestamp",
	"deletionGracePeriodSeconds", "managedFields", "selfLink",
}

// k8sListManifest renders a listed object as the YAML manifest of a list
// result: apiVersion and kind (which list items leave out) added, status and
// the server-managed metadata dropped.
func k8sListManifest(obj map[string]interface{}, apiVersion, kind string) (string, error) {
	m := make(map[string]interface{}, len(obj)+2)
	for k, v := range obj {
		m[k] = v
	}
	delete(m, "status")
	if apiVersion != "" {
		m["apiVersion"] = apiVersion
	}
	if kind != "" {
		m["kind"] = kind
	}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		copied := make(map[string]interface{}, len(meta))
		for k, v := range meta {
			copied[k] = v
		}
		for _, k := range k8sServerManagedMetadata {
			delete(copied, k)
		}
		if ann, ok := copied["annotations"].(map[string]interface{}); ok {
			kept := make(map[string]interface{}, len(ann))
			for k, v := range ann {
				if k != "kubectl.kubernetes.io/last-applied-configuration" {
					kept[k] = v
				}
			}
			if len(kept) == 0 {
				delete(copied, "annotations")
			} else {
				copied["annotations"] = kept
			}
		}
		m["metadata"] = copied
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *k8sManifestListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var diags diag.Diagnostics
	if r.client == nil {
		diags.AddError("Provider not configured", "The list resource needs a configured portainer provider.")
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	items := r.items(ctx, req.Config, &diags)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	res := &k8sManifestResource{kind: r.kind, client: r.client}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.display
			r.kind.setIdentity(ctx, item.id, result.Identity, &result.Diagnostics)
			// The manifest goes into state as authored, so Read compares the
			// live object with it and finds no drift.
			if req.IncludeResource && !result.Diagnostics.HasError() &&
				!fwListReadResource(ctx, res, fwIDListItem{id: item.id, seed: map[string]any{"manifest": item.manifest}}, &result) {
				continue // gone between the listing and the read
			}
			if !push(result) {
				return
			}
		}
	}
}

// The manifest resources with a list resource. Every kind is listed: each is
// a long-lived object addressed by its composite ID.
var (
	newKubernetesApplicationListResource        = newK8sManifestList(k8sKindApplication)
	newKubernetesCronJobListResource            = newK8sManifestList(k8sKindCronJob)
	newKubernetesJobListResource                = newK8sManifestList(k8sKindJob)
	newKubernetesServiceAccountsListResource    = newK8sManifestList(k8sKindServiceAccounts)
	newKubernetesConfigMapsListResource         = newK8sManifestList(k8sKindConfigMaps)
	newKubernetesSecretListResource             = newK8sManifestList(k8sKindSecret)
	newKubernetesServiceListResource            = newK8sManifestList(k8sKindService)
	newKubernetesRoleListResource               = newK8sManifestList(k8sKindRole)
	newKubernetesRoleBindingListResource        = newK8sManifestList(k8sKindRoleBinding)
	newKubernetesClusterRoleListResource        = newK8sManifestList(k8sKindClusterRole)
	newKubernetesClusterRoleBindingListResource = newK8sManifestList(k8sKindClusterRoleBinding)
	newKubernetesVolumeListResource             = newK8sManifestList(k8sKindVolume)
	newKubernetesStorageListResource            = newK8sManifestList(k8sKindStorage)
)

// fwNamedItems turns (id, name) pairs into list items, keeping those whose
// name matches the filter ("" matches all).
func fwNamedItems[T any](all []T, filter string, idName func(T) (string, string), seed map[string]any) []fwIDListItem {
	var out []fwIDListItem
	for _, x := range all {
		id, name := idName(x)
		if filter == "" || name == filter {
			out = append(out, fwIDListItem{id: id, name: name, seed: seed})
		}
	}
	return out
}

// fwAPIItem is the ID and name of an Edge stack or job in their listings.
type fwAPIItem struct {
	ID   int    `json:"Id"`
	Name string `json:"Name"`
}

// edgeStackListReader reads a listed Edge stack the way the read after an
// import does: every argument Portainer reports.
type edgeStackListReader struct{ edgeStackResource }

func (r *edgeStackListReader) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m edgeStackModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &m, &resp.State, true, &resp.Diagnostics)
}

var newEdgeStackListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_edge_stack",
	description: "Lists Portainer Edge stacks. Registry credentials and other write-only settings are never part of the results.",
	newResource: func() resource.Resource { return &edgeStackListReader{} },
	filters:     fwListNameFilter("Edge stacks"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		var stacks []fwAPIItem
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_stacks", nil, &stacks); err != nil {
			diags.AddError("Failed to list edge stacks", fmt.Sprintf("failed to list edge stacks: %s", err))
			return nil
		}
		return fwNamedItems(stacks, name, func(s fwAPIItem) (string, string) { return fwID(int64(s.ID)), s.Name }, nil)
	},
})

var newEdgeJobListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_edge_job",
	description: "Lists Portainer Edge jobs. The script content is not part of the results.",
	newResource: newEdgeJobResource,
	filters:     fwListNameFilter("Edge jobs"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		var jobs []fwAPIItem
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_jobs", nil, &jobs); err != nil {
			diags.AddError("Failed to list edge jobs", fmt.Sprintf("failed to list edge jobs: %s", err))
			return nil
		}
		return fwNamedItems(jobs, name, func(j fwAPIItem) (string, string) { return fwID(int64(j.ID)), j.Name }, nil)
	},
})

var newEdgeConfigurationsListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_edge_configurations",
	description: "Lists Portainer Edge configurations (Business Edition). The uploaded files are not part of the results.",
	newResource: newEdgeConfigurationsResource,
	filters:     fwListNameFilter("Edge configurations"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		configs, err := listEdgeConfigurations(ctx, client)
		if err != nil {
			diags.AddError("Failed to list edge configurations", err.Error())
			return nil
		}
		return fwNamedItems(configs, name, func(c EdgeConfiguration) (string, string) { return fwID(int64(c.ID)), c.Name }, nil)
	},
})

// gitopsSourceListItem is a source in GET /gitops/sources.
type gitopsSourceListItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var newGitopsSourceListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_gitops_source",
	description: "Lists Portainer GitOps sources (Business Edition). Passwords are never part of the results.",
	newResource: newGitopsSourceResource,
	filters:     fwListNameFilter("GitOps sources"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		var sources []gitopsSourceListItem
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/sources", nil, &sources); err != nil {
			gitopsDSAddError(diags, fmt.Errorf("failed to list GitOps sources: %w", err))
			return nil
		}
		// administrators_only is not readable: it starts at its default, as
		// after an import.
		seed := map[string]any{"administrators_only": false}
		return fwNamedItems(sources, name, func(s gitopsSourceListItem) (string, string) { return fwID(int64(s.ID)), s.Name }, seed)
	},
})

// gitopsWorkflowListReader reads a listed GitOps workflow as the read after
// an import does, without the private state marker a list result cannot
// carry.
type gitopsWorkflowListReader struct{ gitopsWorkflowResource }

func (r *gitopsWorkflowListReader) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gitopsWorkflowResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, false, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.Artifact.IsNull() && !state.Artifact.IsUnknown() {
		if l, ok := gitopsNullZeroes(state.Artifact, gitopsArtifactKeptZeroes).(types.List); ok {
			state.Artifact = l
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

var newGitopsWorkflowListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_gitops_workflow",
	description: "Lists Portainer GitOps workflows (Business Edition).",
	newResource: func() resource.Resource { return &gitopsWorkflowListReader{} },
	filters:     fwListNameFilter("GitOps workflows"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		var workflows []gitopsWorkflow
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/gitops/workflows", nil, &workflows); err != nil {
			gitopsDSAddError(diags, fmt.Errorf("failed to list GitOps workflows: %w", err))
			return nil
		}
		return fwNamedItems(workflows, name, func(w gitopsWorkflow) (string, string) { return fwID(int64(w.ID)), w.Name }, nil)
	},
})
