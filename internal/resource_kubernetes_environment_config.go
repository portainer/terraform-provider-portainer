package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// resourceKubernetesEnvironmentConfig manages what the Portainer UI shows as
// the "Kubernetes features configuration" of a Kubernetes environment: load
// balancers, metrics, resource over-commit, the default namespace and the
// storage classes users may pick. Ingress classes are left to
// portainer_kubernetes_ingresscontrollers, which owns them.
func resourceKubernetesEnvironmentConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesEnvironmentConfigApply,
		ReadContext:   resourceKubernetesEnvironmentConfigRead,
		UpdateContext: resourceKubernetesEnvironmentConfigApply,
		DeleteContext: resourceKubernetesEnvironmentConfigDelete,
		Importer: &schema.ResourceImporter{
			StateContext: importKubernetesEnvironmentConfig,
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the Kubernetes environment whose configuration is managed. Changing this value forces resource recreation.",
			},
			"use_load_balancer": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether users may expose applications through a LoadBalancer service.",
			},
			"use_server_metrics": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether Portainer reads metrics from the cluster's metrics server.",
			},
			"enable_resource_over_commit": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether namespace quotas may add up to more than the cluster's capacity.",
			},
			"resource_over_commit_percentage": {
				Type:         schema.TypeInt,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validation.IntBetween(1, 100),
				Description:  "Share of the cluster's resources (1-100) namespaces may reserve when over-commit is disabled.",
			},
			"restrict_default_namespace": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether the `default` namespace is restricted to administrators.",
			},
			"ingress_availability_per_namespace": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether ingress classes can be made available per namespace.",
			},
			"allow_none_ingress_class": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether users may create ingresses without an ingress class.",
			},
			"storage_class": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "Storage classes users may pick for their volumes. When set, the list replaces the enabled storage classes; leave it out to keep the ones configured in Portainer.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":                   {Type: schema.TypeString, Required: true, Description: "Name of the Kubernetes StorageClass."},
						"provisioner":            {Type: schema.TypeString, Required: true, Description: "Provisioner of the StorageClass, e.g. `rancher.io/local-path`."},
						"access_modes":           {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Access modes users may request, e.g. `RWO` or `RWX`."},
						"allow_volume_expansion": {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether volumes of this class may be expanded."},
					},
				},
			},
		},
	}
}

// kubernetesConfigFields maps the schema to KubernetesConfiguration keys.
var kubernetesConfigFields = map[string]string{
	"use_load_balancer":                  "UseLoadBalancer",
	"use_server_metrics":                 "UseServerMetrics",
	"enable_resource_over_commit":        "EnableResourceOverCommit",
	"resource_over_commit_percentage":    "ResourceOverCommitPercentage",
	"restrict_default_namespace":         "RestrictDefaultNamespace",
	"ingress_availability_per_namespace": "IngressAvailabilityPerNamespace",
	"allow_none_ingress_class":           "AllowNoneIngressClass",
}

// kubernetesConfigLocks serializes, per environment, the calls that change
// its Kubernetes configuration. This resource reads the whole object and
// writes it back, so an ingress controller update Terraform runs in parallel
// (it stores into the same object) would otherwise be overwritten.
var kubernetesConfigLocks sync.Map

// lockKubernetesConfig locks the environment's Kubernetes configuration and
// returns the unlock function.
func lockKubernetesConfig(endpointID int) func() {
	m, _ := kubernetesConfigLocks.LoadOrStore(endpointID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// getKubernetesData returns the environment's Kubernetes object as Portainer
// stores it, or nil when the environment is gone.
func getKubernetesData(ctx context.Context, client *APIClient, endpointID int) (map[string]interface{}, error) {
	var endpoint struct {
		Kubernetes map[string]interface{} `json:"Kubernetes"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d", client.Endpoint, endpointID), nil, &endpoint); err != nil {
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read environment %d: %w", endpointID, err)
	}
	if endpoint.Kubernetes == nil {
		endpoint.Kubernetes = map[string]interface{}{}
	}
	return endpoint.Kubernetes, nil
}

func resourceKubernetesEnvironmentConfigApply(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	// PUT /endpoints/{id} replaces the whole Kubernetes object, snapshots and
	// detected flags included, so it is read first and only its configuration
	// is changed - the way the Portainer UI does it.
	unlock := lockKubernetesConfig(endpointID)
	defer unlock()
	kube, err := getKubernetesData(ctx, client, endpointID)
	if err != nil {
		return diag.FromErr(err)
	}
	if kube == nil {
		return diag.Errorf("environment %d not found", endpointID)
	}
	config, _ := kube["Configuration"].(map[string]interface{})
	if config == nil {
		config = map[string]interface{}{}
	}
	for key, field := range kubernetesConfigFields {
		// Only configured attributes are sent; the rest keep Portainer's
		// value. Writing d.Get back for everything sent false and 0 for every
		// attribute left out on create, where nothing is in state yet.
		if v, ok := configuredValue(d, key); ok {
			config[field] = v
		}
	}
	if v, ok := d.GetOk("storage_class"); ok {
		classes := make([]map[string]interface{}, 0, len(v.([]interface{})))
		for _, raw := range v.([]interface{}) {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			classes = append(classes, map[string]interface{}{
				"Name":                 m["name"],
				"Provisioner":          m["provisioner"],
				"AccessModes":          m["access_modes"],
				"AllowVolumeExpansion": m["allow_volume_expansion"],
			})
		}
		config["StorageClasses"] = classes
	}
	kube["Configuration"] = config

	url := fmt.Sprintf("%s/endpoints/%d", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, url, map[string]interface{}{"Kubernetes": kube}, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the Kubernetes configuration of environment %d: %w", endpointID, err))
	}
	d.SetId(strconv.Itoa(endpointID))
	return resourceKubernetesEnvironmentConfigRead(ctx, d, meta)
}

// configuredValue returns the attribute's value when the configuration sets
// it. Without a raw configuration (unit tests) a non-zero value counts as set.
func configuredValue(d *schema.ResourceData, key string) (interface{}, bool) {
	if d.GetRawConfig().IsNull() {
		return d.GetOk(key)
	}
	raw, diags := d.GetRawConfigAt(cty.GetAttrPath(key))
	if diags.HasError() || !raw.IsKnown() || raw.IsNull() {
		return nil, false
	}
	return d.Get(key), true
}

func resourceKubernetesEnvironmentConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	endpointID := d.Get("endpoint_id").(int)
	kube, err := getKubernetesData(ctx, meta.(*APIClient), endpointID)
	if err != nil {
		return diag.FromErr(err)
	}
	if kube == nil {
		d.SetId("")
		return nil
	}
	config, _ := kube["Configuration"].(map[string]interface{})

	fields := map[string]interface{}{}
	for key, field := range kubernetesConfigFields {
		switch v := config[field].(type) {
		case bool:
			fields[key] = v
		case float64:
			fields[key] = int(v)
		default:
			if key == "resource_over_commit_percentage" {
				fields[key] = 0
			} else {
				fields[key] = false
			}
		}
	}
	classes := []map[string]interface{}{}
	rawClasses, _ := config["StorageClasses"].([]interface{})
	for _, raw := range rawClasses {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["Name"].(string)
		provisioner, _ := m["Provisioner"].(string)
		expansion, _ := m["AllowVolumeExpansion"].(bool)
		modes := []string{}
		rawModes, _ := m["AccessModes"].([]interface{})
		for _, mode := range rawModes {
			if s, ok := mode.(string); ok {
				modes = append(modes, s)
			}
		}
		classes = append(classes, map[string]interface{}{
			"name":                   name,
			"provisioner":            provisioner,
			"access_modes":           modes,
			"allow_volume_expansion": expansion,
		})
	}
	fields["storage_class"] = classes
	return diag.FromErr(setFields(d, fields))
}

// resourceKubernetesEnvironmentConfigDelete only forgets the configuration:
// an environment always has one, and there is no state to return to.
func resourceKubernetesEnvironmentConfigDelete(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

// importKubernetesEnvironmentConfig takes the environment ID.
func importKubernetesEnvironmentConfig(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	if err := setImportInts(d, d.Id(), "<endpoint_id>", map[string]string{"endpoint_id": d.Id()}); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
