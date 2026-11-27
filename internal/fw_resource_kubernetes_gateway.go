package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Gateway API objects are managed through Portainer's own Kubernetes API, the
// way portainer_kubernetes_ingresses manages Ingresses: Portainer takes a flat
// PascalCase model (K8sGateway, K8sGatewayHTTPRoute) and writes the
// gateway.networking.k8s.io objects itself. That API has no GET for a single
// object, so Read lists the namespace and picks the object by name. Unlike an
// Ingress, the listed object carries the same model that was written, so Read
// refreshes every argument and drift is detected, not only deletion.

const gatewayAPIIDFormat = "<environment_id>:<namespace>:<name>"

// k8sGateway is kubernetes.K8sGateway from the Portainer 3.0 API.
type k8sGateway struct {
	Name            string               `json:"Name"`
	Namespace       string               `json:"Namespace"`
	ClassName       string               `json:"ClassName"`
	Listeners       []k8sGatewayListener `json:"Listeners"`
	Addresses       []k8sGatewayAddress  `json:"Addresses"`
	Annotations     map[string]string    `json:"Annotations"`
	Labels          map[string]string    `json:"Labels"`
	ResourceVersion string               `json:"ResourceVersion,omitempty"`
	UID             string               `json:"UID,omitempty"`
	CreationDate    string               `json:"CreationDate,omitempty"`
}

type k8sGatewayListener struct {
	Name     string `json:"Name"`
	Hostname string `json:"Hostname,omitempty"`
	Port     int    `json:"Port"`
	Protocol string `json:"Protocol"`
}

type k8sGatewayAddress struct {
	Type  string `json:"Type,omitempty"`
	Value string `json:"Value"`
}

var (
	_ resource.Resource                = (*kubernetesGatewayResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesGatewayResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesGatewayResource)(nil)
)

func newKubernetesGatewayResource() resource.Resource {
	return &kubernetesGatewayResource{}
}

type kubernetesGatewayResource struct{ k8sfwBase }

func (r *kubernetesGatewayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_gateway"
}

func (r *kubernetesGatewayResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the Gateway, `" + gatewayAPIIDFormat + "`.",
				PlanModifiers: keep,
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment where the Gateway is managed.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Kubernetes namespace in which the Gateway is created.",
				PlanModifiers: replace,
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Gateway.",
				PlanModifiers: replace,
			},
			"class_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the GatewayClass implementing this Gateway (sets `spec.gatewayClassName`). See the `portainer_kubernetes_gateway_classes` data source.",
			},
			"annotations": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Annotations applied to the Gateway as key/value strings.",
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Labels applied to the Gateway as key/value strings.",
			},
			"uid": schema.StringAttribute{
				Computed:      true,
				Description:   "Kubernetes UID of the Gateway.",
				PlanModifiers: keep,
			},
			"resource_version": schema.StringAttribute{
				Computed:    true,
				Description: "Kubernetes resource version of the Gateway, sent back on update so a concurrent change is rejected instead of overwritten.",
			},
			"creation_date": schema.StringAttribute{
				Computed:      true,
				Description:   "Time the Gateway was created, as reported by Portainer.",
				PlanModifiers: keep,
			},
		},
		Blocks: map[string]schema.Block{
			"listener": schema.ListNestedBlock{
				Description: "Listeners of the Gateway: the ports, protocols and hostnames it accepts traffic on.",
				Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the listener, unique within the Gateway. An HTTPRoute attaches to it through `parent_ref.section_name`.",
						},
						"port": schema.Int64Attribute{
							Required:    true,
							Description: "Network port the listener accepts traffic on.",
						},
						"protocol": schema.StringAttribute{
							Required:    true,
							Description: "Protocol the listener accepts, for example `HTTP` or `HTTPS`.",
						},
						"hostname": schema.StringAttribute{
							Optional:    true,
							Description: "Hostname the listener matches, for example `*.example.com`. Leave unset to match every hostname.",
						},
					},
				},
			},
			"address": schema.ListNestedBlock{
				Description: "Addresses requested for the Gateway (`spec.addresses`). Leave unset to let the Gateway implementation assign one.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Optional:    true,
							Description: "Type of the address, for example `IPAddress` or `Hostname`.",
						},
						"value": schema.StringAttribute{
							Required:    true,
							Description: "The address itself.",
						},
					},
				},
			},
		},
	}
}

// parseGatewayAPIID splits "<environment_id>:<namespace>:<name>", the ID of
// both Gateway API resources.
func parseGatewayAPIID(id string) (int, string, string, error) {
	parts, err := importIDParts(id, ":", 3, gatewayAPIIDFormat)
	if err != nil {
		return 0, "", "", err
	}
	envID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid ID %q: expected %s", id, gatewayAPIIDFormat)
	}
	return envID, parts[1], parts[2], nil
}

func buildKubernetesGateway(data map[string]interface{}) k8sGateway {
	str := func(m map[string]interface{}, k string) string { s, _ := m[k].(string); return s }
	gw := k8sGateway{
		Name:            str(data, "name"),
		Namespace:       str(data, "namespace"),
		ClassName:       str(data, "class_name"),
		Listeners:       []k8sGatewayListener{},
		Addresses:       []k8sGatewayAddress{},
		Annotations:     k8sfwStringMap(data["annotations"]),
		Labels:          k8sfwStringMap(data["labels"]),
		ResourceVersion: str(data, "resource_version"),
	}
	for _, raw := range k8sfwList(data, "listener") {
		m, _ := raw.(map[string]interface{})
		port, _ := m["port"].(int)
		gw.Listeners = append(gw.Listeners, k8sGatewayListener{
			Name:     str(m, "name"),
			Hostname: str(m, "hostname"),
			Port:     port,
			Protocol: str(m, "protocol"),
		})
	}
	for _, raw := range k8sfwList(data, "address") {
		m, _ := raw.(map[string]interface{})
		gw.Addresses = append(gw.Addresses, k8sGatewayAddress{
			Type:  str(m, "type"),
			Value: str(m, "value"),
		})
	}
	return gw
}

func gatewaysURL(client *APIClient, envID int, namespace string) string {
	return fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/gateways", client.Endpoint, envID, namespace)
}

// findGateway lists the namespace and returns the Gateway, nil when it or
// the environment is gone.
func (r *kubernetesGatewayResource) findGateway(ctx context.Context, envID int, namespace, name string) (*k8sGateway, error) {
	var gateways []k8sGateway
	if err := doJSON(ctx, r.client, http.MethodGet, gatewaysURL(r.client, envID, namespace), nil, &gateways); err != nil {
		// The environment itself is gone.
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read gateway %s/%s: %w", namespace, name, err)
	}
	for i := range gateways {
		if gateways[i].Name == name {
			return &gateways[i], nil
		}
	}
	return nil, nil
}

// read refreshes data from the Gateway, reporting false when it is gone.
// Only the annotation and label keys data holds are read back (all of them
// but Portainer's own right after an import), see k8sManagedMetadata.
func (r *kubernetesGatewayResource) read(ctx context.Context, data map[string]interface{}, imported bool) (bool, error) {
	id, _ := data["id"].(string)
	envID, namespace, name, err := parseGatewayAPIID(id)
	if err != nil {
		return false, err
	}
	gw, err := r.findGateway(ctx, envID, namespace, name)
	if err != nil || gw == nil {
		return false, err
	}

	listeners := make([]interface{}, 0, len(gw.Listeners))
	for _, l := range gw.Listeners {
		listeners = append(listeners, map[string]interface{}{
			"name":     l.Name,
			"hostname": l.Hostname,
			"port":     l.Port,
			"protocol": l.Protocol,
		})
	}
	addresses := make([]interface{}, 0, len(gw.Addresses))
	for _, a := range gw.Addresses {
		addresses = append(addresses, map[string]interface{}{
			"type":  a.Type,
			"value": a.Value,
		})
	}

	data["environment_id"] = envID
	data["namespace"] = namespace
	data["name"] = name
	data["class_name"] = gw.ClassName
	data["listener"] = listeners
	data["address"] = addresses
	data["annotations"] = k8sManagedMetadata(gw.Annotations, k8sfwMap(data, "annotations"), imported)
	data["labels"] = k8sManagedMetadata(gw.Labels, k8sfwMap(data, "labels"), imported)
	data["uid"] = gw.UID
	data["resource_version"] = gw.ResourceVersion
	data["creation_date"] = gw.CreationDate
	return true, nil
}

func (r *kubernetesGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	envID, _ := data["environment_id"].(int)
	gw := buildKubernetesGateway(data)
	gw.ResourceVersion = ""

	if err := doJSON(ctx, r.client, http.MethodPost, gatewaysURL(r.client, envID, gw.Namespace), gw, nil); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_kubernetes_gateway", requiresBE30)
		resp.Diagnostics.AddError("Failed to create the gateway", fmt.Errorf("failed to create gateway %s/%s: %w", gw.Namespace, gw.Name, err).Error())
		return
	}

	data["id"] = fmt.Sprintf("%d:%s:%s", envID, gw.Namespace, gw.Name)
	found, err := r.read(ctx, data, false)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the gateway", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the gateway", fmt.Sprintf("gateway %s/%s not found after it was created", gw.Namespace, gw.Name))
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesGatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	imported := k8sImported(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, data, imported)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the gateway", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
	k8sClearImported(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	before, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := before["id"].(string)
	envID, namespace, name, err := parseGatewayAPIID(id)
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}
	// The resource version is the one last read, so a concurrent change is
	// rejected instead of overwritten.
	data["id"] = id
	data["resource_version"] = before["resource_version"]

	// The keys someone else added are sent back as they are now; Portainer
	// replaces the whole map, which would delete them.
	live, err := r.findGateway(ctx, envID, namespace, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the gateway", err.Error())
		return
	}
	gw := buildKubernetesGateway(data)
	if live != nil {
		gw.Annotations = k8sMergeForeignMetadata(live.Annotations, k8sfwMap(before, "annotations"), k8sfwMap(data, "annotations"))
		gw.Labels = k8sMergeForeignMetadata(live.Labels, k8sfwMap(before, "labels"), k8sfwMap(data, "labels"))
	}

	if err := doJSON(ctx, r.client, http.MethodPut, gatewaysURL(r.client, envID, namespace), gw, nil); err != nil {
		resp.Diagnostics.AddError("Failed to update the gateway", fmt.Errorf("failed to update gateway %s/%s: %w", namespace, name, err).Error())
		return
	}
	found, err := r.read(ctx, data, false)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the gateway", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the gateway", fmt.Sprintf("gateway %s/%s not found after it was updated", namespace, name))
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete removes the Gateway through the bulk delete, which takes the names
// to delete per namespace.
func (r *kubernetesGatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID, namespace, name, err := parseGatewayAPIID(id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	body := map[string][]string{namespace: {name}}
	url := fmt.Sprintf("%s/kubernetes/%d/gateways/delete", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, body, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete the gateway", fmt.Errorf("failed to delete gateway %s/%s: %w", namespace, name, err).Error())
	}
}

// ImportState takes "<environment_id>:<namespace>:<name>". The first Read
// takes every annotation and label but Portainer's own, as there is no
// configuration yet to tell the managed ones apart.
func (r *kubernetesGatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	k8sMarkImported(ctx, resp.Private, &resp.Diagnostics)
}
