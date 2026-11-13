package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

func resourceKubernetesGateway() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesGatewayCreate,
		ReadContext:   resourceKubernetesGatewayRead,
		UpdateContext: resourceKubernetesGatewayUpdate,
		DeleteContext: resourceKubernetesGatewayDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the Gateway is managed.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace in which the Gateway is created.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Gateway.",
			},
			"class_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the GatewayClass implementing this Gateway (sets `spec.gatewayClassName`). See the `portainer_kubernetes_gateway_classes` data source.",
			},
			"listener": {
				Type:        schema.TypeList,
				Required:    true,
				MinItems:    1,
				Description: "Listeners of the Gateway: the ports, protocols and hostnames it accepts traffic on.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the listener, unique within the Gateway. An HTTPRoute attaches to it through `parent_ref.section_name`.",
						},
						"port": {
							Type:        schema.TypeInt,
							Required:    true,
							Description: "Network port the listener accepts traffic on.",
						},
						"protocol": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Protocol the listener accepts, for example `HTTP` or `HTTPS`.",
						},
						"hostname": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Hostname the listener matches, for example `*.example.com`. Leave unset to match every hostname.",
						},
					},
				},
			},
			"address": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Addresses requested for the Gateway (`spec.addresses`). Leave unset to let the Gateway implementation assign one.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Type of the address, for example `IPAddress` or `Hostname`.",
						},
						"value": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The address itself.",
						},
					},
				},
			},
			"annotations": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Annotations applied to the Gateway as key/value strings.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels applied to the Gateway as key/value strings.",
			},
			// Computed attributes
			"uid": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes UID of the Gateway.",
			},
			"resource_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Kubernetes resource version of the Gateway, sent back on update so a concurrent change is rejected instead of overwritten.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Time the Gateway was created, as reported by Portainer.",
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

func buildKubernetesGateway(d *schema.ResourceData) k8sGateway {
	gw := k8sGateway{
		Name:            d.Get("name").(string),
		Namespace:       d.Get("namespace").(string),
		ClassName:       d.Get("class_name").(string),
		Listeners:       []k8sGatewayListener{},
		Addresses:       []k8sGatewayAddress{},
		Annotations:     convertMapString(d.Get("annotations").(map[string]interface{})),
		Labels:          convertMapString(d.Get("labels").(map[string]interface{})),
		ResourceVersion: d.Get("resource_version").(string),
	}
	for _, raw := range d.Get("listener").([]interface{}) {
		m := raw.(map[string]interface{})
		gw.Listeners = append(gw.Listeners, k8sGatewayListener{
			Name:     m["name"].(string),
			Hostname: m["hostname"].(string),
			Port:     m["port"].(int),
			Protocol: m["protocol"].(string),
		})
	}
	for _, raw := range d.Get("address").([]interface{}) {
		m := raw.(map[string]interface{})
		gw.Addresses = append(gw.Addresses, k8sGatewayAddress{
			Type:  m["type"].(string),
			Value: m["value"].(string),
		})
	}
	return gw
}

// withoutPortainerKeys drops the io.portainer.* keys Portainer adds itself,
// such as the io.portainer.kubernetes.gateway.owner label: they are not part
// of the configuration and would show as drift on every plan.
func withoutPortainerKeys(source map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(source))
	for k, v := range source {
		if !strings.HasPrefix(k, "io.portainer.") {
			out[k] = v
		}
	}
	return out
}

func gatewaysURL(client *APIClient, envID int, namespace string) string {
	return fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/gateways", client.Endpoint, envID, namespace)
}

func resourceKubernetesGatewayCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	gw := buildKubernetesGateway(d)
	gw.ResourceVersion = ""

	if err := doJSON(ctx, client, http.MethodPost, gatewaysURL(client, envID, gw.Namespace), gw, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_gateway", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to create gateway %s/%s: %w", gw.Namespace, gw.Name, err))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", envID, gw.Namespace, gw.Name))
	return resourceKubernetesGatewayRead(ctx, d, meta)
}

func resourceKubernetesGatewayRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var gateways []k8sGateway
	if err := doJSON(ctx, client, http.MethodGet, gatewaysURL(client, envID, namespace), nil, &gateways); err != nil {
		// The environment itself is gone.
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read gateway %s/%s: %w", namespace, name, err))
	}

	var gw *k8sGateway
	for i := range gateways {
		if gateways[i].Name == name {
			gw = &gateways[i]
			break
		}
	}
	if gw == nil {
		d.SetId("")
		return nil
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

	if err := setFields(d, map[string]interface{}{
		"environment_id":   envID,
		"namespace":        namespace,
		"name":             name,
		"class_name":       gw.ClassName,
		"listener":         listeners,
		"address":          addresses,
		"annotations":      withoutPortainerKeys(gw.Annotations),
		"labels":           withoutPortainerKeys(gw.Labels),
		"uid":              gw.UID,
		"resource_version": gw.ResourceVersion,
		"creation_date":    gw.CreationDate,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceKubernetesGatewayUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := doJSON(ctx, client, http.MethodPut, gatewaysURL(client, envID, namespace), buildKubernetesGateway(d), nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update gateway %s/%s: %w", namespace, name, err))
	}
	return resourceKubernetesGatewayRead(ctx, d, meta)
}

// resourceKubernetesGatewayDelete removes the Gateway through the bulk
// delete, which takes the names to delete per namespace.
func resourceKubernetesGatewayDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID, namespace, name, err := parseGatewayAPIID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	body := map[string][]string{namespace: {name}}
	url := fmt.Sprintf("%s/kubernetes/%d/gateways/delete", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil && !isAPINotFound(err) {
		return diag.FromErr(fmt.Errorf("failed to delete gateway %s/%s: %w", namespace, name, err))
	}
	d.SetId("")
	return nil
}
