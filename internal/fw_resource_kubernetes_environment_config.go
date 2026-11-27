package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesEnvironmentConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesEnvironmentConfigResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesEnvironmentConfigResource)(nil)
)

// kubernetesEnvironmentConfigResource manages what the Portainer UI shows as
// the "Kubernetes features configuration" of a Kubernetes environment: load
// balancers, metrics, resource over-commit, the default namespace and the
// storage classes users may pick. Ingress classes are left to
// portainer_kubernetes_ingresscontrollers, which owns them.
func newKubernetesEnvironmentConfigResource() resource.Resource {
	return &kubernetesEnvironmentConfigResource{}
}

type kubernetesEnvironmentConfigResource struct{ k8sfwBase }

func (r *kubernetesEnvironmentConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_environment_config"
}

func (r *kubernetesEnvironmentConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the environment whose configuration is managed.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "ID of the Kubernetes environment whose configuration is managed. Changing this value forces resource recreation.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"use_load_balancer": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether users may expose applications through a LoadBalancer service.",
				PlanModifiers: keep,
			},
			"use_server_metrics": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Portainer reads metrics from the cluster's metrics server.",
				PlanModifiers: keep,
			},
			"enable_resource_over_commit": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether namespace quotas may add up to more than the cluster's capacity.",
				PlanModifiers: keep,
			},
			"resource_over_commit_percentage": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Share of the cluster's resources (1-100) namespaces may reserve when over-commit is disabled.",
				Validators:    []validator.Int64{int64validator.Between(1, 100)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"restrict_default_namespace": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the `default` namespace is restricted to administrators.",
				PlanModifiers: keep,
			},
			"ingress_availability_per_namespace": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether ingress classes can be made available per namespace.",
				PlanModifiers: keep,
			},
			"allow_none_ingress_class": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether users may create ingresses without an ingress class.",
				PlanModifiers: keep,
			},
		},
		Blocks: map[string]schema.Block{
			// Optional and computed in the SDKv2 version. A block cannot be
			// computed in the framework: left out, it is empty in the plan and
			// in state, and Read leaves it empty instead of reporting the
			// classes configured in Portainer, which this resource then does
			// not touch.
			"storage_class": schema.ListNestedBlock{
				Description: "Storage classes users may pick for their volumes. When set, the list replaces the enabled storage classes; leave it out to keep the ones configured in Portainer.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":        schema.StringAttribute{Required: true, Description: "Name of the Kubernetes StorageClass."},
						"provisioner": schema.StringAttribute{Required: true, Description: "Provisioner of the StorageClass, e.g. `rancher.io/local-path`."},
						"access_modes": schema.ListAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Access modes users may request, e.g. `RWO` or `RWX`.",
						},
						"allow_volume_expansion": schema.BoolAttribute{
							Optional:      true,
							Computed:      true,
							Default:       booldefault.StaticBool(false),
							Description:   "Whether volumes of this class may be expanded.",
							PlanModifiers: []planmodifier.Bool{k8sfwKeepNull{}},
						},
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

func (r *kubernetesEnvironmentConfigResource) apply(ctx context.Context, data map[string]interface{}, configured func(string) bool, diags *diag.Diagnostics) {
	endpointID, _ := data["endpoint_id"].(int)

	// PUT /endpoints/{id} replaces the whole Kubernetes object, snapshots and
	// detected flags included, so it is read first and only its configuration
	// is changed - the way the Portainer UI does it.
	unlock := lockKubernetesConfig(endpointID)
	defer unlock()
	kube, err := getKubernetesData(ctx, r.client, endpointID)
	if err != nil {
		diags.AddError("Failed to update the Kubernetes configuration", err.Error())
		return
	}
	if kube == nil {
		diags.AddError("Failed to update the Kubernetes configuration", fmt.Sprintf("environment %d not found", endpointID))
		return
	}
	config, _ := kube["Configuration"].(map[string]interface{})
	if config == nil {
		config = map[string]interface{}{}
	}
	for key, field := range kubernetesConfigFields {
		// Only configured attributes are sent; the rest keep Portainer's
		// value. Writing the planned value back for everything sent false
		// and 0 for every attribute left out on create.
		if configured(key) {
			config[field] = data[key]
		}
	}
	if v, ok := k8sfwGetOk(data, "storage_class"); ok {
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

	url := fmt.Sprintf("%s/endpoints/%d", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodPut, url, map[string]interface{}{"Kubernetes": kube}, nil); err != nil {
		diags.AddError("Failed to update the Kubernetes configuration", fmt.Errorf("failed to update the Kubernetes configuration of environment %d: %w", endpointID, err).Error())
		return
	}
	data["id"] = strconv.Itoa(endpointID)
	if _, err := r.read(ctx, data, true); err != nil {
		diags.AddError("Failed to read the Kubernetes configuration", err.Error())
	}
}

// read refreshes data from the environment, reporting false when it is
// gone. The storage classes are refreshed only when they are managed.
func (r *kubernetesEnvironmentConfigResource) read(ctx context.Context, data map[string]interface{}, classesManaged bool) (bool, error) {
	endpointID, _ := data["endpoint_id"].(int)
	kube, err := getKubernetesData(ctx, r.client, endpointID)
	if err != nil {
		return false, err
	}
	if kube == nil {
		return false, nil
	}
	config, _ := kube["Configuration"].(map[string]interface{})

	for key, field := range kubernetesConfigFields {
		switch v := config[field].(type) {
		case bool:
			data[key] = v
		case float64:
			data[key] = int(v)
		default:
			if key == "resource_over_commit_percentage" {
				data[key] = 0
			} else {
				data[key] = false
			}
		}
	}
	if !classesManaged {
		return true, nil
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
	data["storage_class"] = classes
	return true, nil
}

func (r *kubernetesEnvironmentConfigResource) configured(ctx context.Context, config k8sfwGetter, diags *diag.Diagnostics) func(string) bool {
	attrs := k8sfwAttrs(k8sfwObject(ctx, config, diags))
	return func(key string) bool {
		v, ok := attrs[key]
		return ok && !v.IsNull() && !v.IsUnknown()
	}
}

func (r *kubernetesEnvironmentConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	configured := r.configured(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, data, configured, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesEnvironmentConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// The storage classes are managed when state has some. A state written
	// by SDKv2, or an import, has no way to tell, and takes them all.
	classes := k8sfwAttrs(prior)["storage_class"]
	managed := len(k8sfwElems(classes)) > 0 || (fromSDK && (classes == nil || classes.IsNull()))
	found, err := r.read(ctx, data, managed)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the Kubernetes configuration", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesEnvironmentConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	configured := r.configured(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, data, configured, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the configuration: an environment always has one, and
// there is no state to return to.
func (r *kubernetesEnvironmentConfigResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

// ImportState takes the environment ID.
func (r *kubernetesEnvironmentConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected <endpoint_id>", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_id"), int64(id))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("storage_class"), types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "provisioner": types.StringType,
		"access_modes": types.ListType{ElemType: types.StringType}, "allow_volume_expansion": types.BoolType,
	}}))...)
}
