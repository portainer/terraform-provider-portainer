package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesIngressResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesIngressResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesIngressResource)(nil)
)

func newKubernetesIngressResource() resource.Resource {
	return &kubernetesIngressResource{}
}

type kubernetesIngressResource struct{ k8sfwBase }

func (r *kubernetesIngressResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_ingresses"
}

func (r *kubernetesIngressResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the Ingress, `<environment_id>:<namespace>:<name>`.",
			},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the Ingress resource is managed.",
			},
			"namespace": schema.StringAttribute{
				Required:    true,
				Description: "Kubernetes namespace in which the Ingress is created.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Kubernetes Ingress resource.",
			},
			"class_name": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the IngressClass that should handle this Ingress (sets `spec.ingressClassName`).",
			},
			"hosts": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "List of hostnames associated with the Ingress.",
			},
			"annotations": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Annotations applied to the Ingress resource as key/value strings.",
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Labels applied to the Ingress resource as key/value strings.",
			},
		},
		Blocks: map[string]schema.Block{
			"tls": schema.ListNestedBlock{
				Description: "TLS configuration blocks for the Ingress, each pairing a list of hosts with a TLS secret.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"hosts": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "List of hostnames covered by the referenced TLS secret.",
						},
						"secret_name": schema.StringAttribute{
							Optional:    true,
							Description: "Name of the Kubernetes Secret containing the TLS certificate and key.",
						},
					},
				},
			},
			"paths": schema.ListNestedBlock{
				Description: "List of routing rules mapping host/path combinations to backend Kubernetes services.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"host":         schema.StringAttribute{Required: true, Description: "Hostname matched by this routing rule."},
						"path":         schema.StringAttribute{Required: true, Description: "URL path matched by this routing rule."},
						"path_type":    schema.StringAttribute{Required: true, Description: "Path matching strategy (`Exact`, `Prefix`, or `ImplementationSpecific`)."},
						"port":         schema.Int64Attribute{Required: true, Description: "Port on the backend Service that traffic is forwarded to."},
						"service_name": schema.StringAttribute{Required: true, Description: "Name of the Kubernetes Service that receives traffic for this rule."},
					},
				},
			},
		},
	}
}

// liveIngress reads the Ingress through the Kubernetes proxy (standard
// networking.k8s.io path), which gives clean 404s.
func (r *kubernetesIngressResource) liveIngress(ctx context.Context, envID int, namespace, name string) (*k8sIngressObject, error) {
	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/networking.k8s.io/v1/namespaces/%s/ingresses/%s", r.client.Endpoint, envID, namespace, name)
	var live k8sIngressObject
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &live); err != nil {
		return nil, err
	}
	return &live, nil
}

// write creates (POST) or replaces (PUT) the Ingress. Portainer replaces the
// whole annotation map, so annotations, when not nil, are sent instead of the
// configured ones (an update merges in the keys owned by someone else).
func (r *kubernetesIngressResource) write(ctx context.Context, data map[string]interface{}, method string, annotations map[string]string, diags *diag.Diagnostics) {
	envID, _ := data["environment_id"].(int)
	namespace, _ := data["namespace"].(string)
	name, _ := data["name"].(string)

	tlsItems := k8sfwList(data, "tls")
	tls := make([]map[string]interface{}, 0, len(tlsItems))
	for _, item := range tlsItems {
		m, _ := item.(map[string]interface{})
		if m == nil {
			m = map[string]interface{}{"hosts": []interface{}{}, "secret_name": ""}
		}
		tls = append(tls, map[string]interface{}{
			"Hosts":      m["hosts"],
			"SecretName": m["secret_name"],
		})
	}

	pathItems := k8sfwList(data, "paths")
	paths := make([]map[string]interface{}, 0, len(pathItems))
	for _, item := range pathItems {
		m, _ := item.(map[string]interface{})
		paths = append(paths, map[string]interface{}{
			"HasService":  true,
			"Host":        m["host"],
			"IngressName": name,
			"Path":        m["path"],
			"PathType":    m["path_type"],
			"Port":        m["port"],
			"ServiceName": m["service_name"],
		})
	}

	hosts := k8sfwList(data, "hosts")
	if hosts == nil {
		hosts = []interface{}{}
	}
	body := map[string]interface{}{
		"Name":        name,
		"Namespace":   namespace,
		"ClassName":   data["class_name"],
		"Annotations": annotationsOrConfigured(annotations, data),
		"Labels":      k8sfwStringMap(data["labels"]),
		"Hosts":       hosts,
		"TLS":         tls,
		"Paths":       paths,
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresses", r.client.Endpoint, envID, namespace)
	if err := doJSON(ctx, r.client, method, url, body, nil); err != nil {
		diags.AddError("Failed to "+strings.ToLower(method)+" the ingress", fmt.Errorf("failed to %s ingress: %w", strings.ToLower(method), err).Error())
		return
	}
	data["id"] = fmt.Sprintf("%d:%s:%s", envID, namespace, name)
}

func (r *kubernetesIngressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, http.MethodPost, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesIngressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	imported := k8sImported(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := data["id"].(string)
	invalid := fmt.Sprintf("invalid ID format, expected 'envID:namespace:name': %s", id)
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid resource ID", invalid)
		return
	}
	var envID int
	_, _ = fmt.Sscanf(parts[0], "%d", &envID)
	namespace := parts[1]
	name := parts[2]
	if envID == 0 || namespace == "" || name == "" {
		resp.Diagnostics.AddError("Invalid resource ID", invalid)
		return
	}

	live, err := r.liveIngress(ctx, envID, namespace, name)
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the ingress", fmt.Errorf("failed to read %s: %w", "ingress "+name, err).Error())
		return
	}

	data["environment_id"] = envID
	data["namespace"] = namespace
	data["name"] = name
	refreshIngressFromLive(data, live, imported)
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
	k8sClearImported(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesIngressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	before, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only the configured annotation keys are managed: the keys someone else
	// added (controllers, cert-manager, kubectl) are sent back as they are
	// now, since Portainer replaces the whole map. A key removed from the
	// configuration (in the prior state, not in the plan) is dropped.
	envID, _ := data["environment_id"].(int)
	namespace, _ := data["namespace"].(string)
	name, _ := data["name"].(string)
	var annotations map[string]string
	live, err := r.liveIngress(ctx, envID, namespace, name)
	switch {
	case err == nil:
		annotations = k8sMergeForeignMetadata(live.Metadata.Annotations, k8sfwMap(before, "annotations"), k8sfwMap(data, "annotations"))
	case !isAPINotFound(err):
		resp.Diagnostics.AddError("Failed to read the ingress", fmt.Errorf("failed to read ingress %s/%s: %w", namespace, name, err).Error())
		return
	}
	r.write(ctx, data, http.MethodPut, annotations, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete removes the ingress through POST /kubernetes/{id}/ingresses/delete,
// which takes the names to delete per namespace.
func (r *kubernetesIngressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parts := strings.SplitN(id.ValueString(), ":", 3)
	envID, err := strconv.Atoi(parts[0])
	if len(parts) != 3 || err != nil || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("invalid ID format, expected 'envID:namespace:name': %s", id.ValueString()))
		return
	}
	namespace, name := parts[1], parts[2]

	body := map[string][]string{namespace: {name}}
	url := fmt.Sprintf("%s/kubernetes/%d/ingresses/delete", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, body, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete the ingress", fmt.Errorf("failed to delete ingress %s/%s: %w", namespace, name, err).Error())
	}
}

// ImportState takes "<environment_id>:<namespace>:<name>". The first Read
// fills the arguments from the Ingress (see refreshIngressFromLive).
func (r *kubernetesIngressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	k8sMarkImported(ctx, resp.Private, &resp.Diagnostics)
}

// annotationsOrConfigured returns merged when set, else the configured
// annotations.
func annotationsOrConfigured(merged map[string]string, data map[string]interface{}) interface{} {
	if merged != nil {
		return merged
	}
	return k8sfwStringMap(data["annotations"])
}
