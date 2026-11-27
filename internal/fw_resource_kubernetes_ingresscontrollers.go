package internal

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type IngressController struct {
	Availability bool   `json:"Availability"`
	ClassName    string `json:"ClassName"`
	Name         string `json:"Name"`
	New          bool   `json:"New"`
	Type         string `json:"Type"`
	Used         bool   `json:"Used"`
}

var (
	_ resource.Resource                = (*kubernetesIngressControllersResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesIngressControllersResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesIngressControllersResource)(nil)
)

func newKubernetesIngressControllersResource() resource.Resource {
	return &kubernetesIngressControllersResource{}
}

type kubernetesIngressControllersResource struct{ k8sfwBase }

func (r *kubernetesIngressControllersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_ingresscontrollers"
}

func (r *kubernetesIngressControllersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the environment whose ingress controllers are managed.",
			},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment whose ingress controllers are managed.",
			},
		},
		Blocks: map[string]schema.Block{
			"controllers": ingressControllersBlock("List of ingress controller configurations to apply at the cluster level."),
		},
	}
}

// ingressControllersBlock is the controllers block of the ingresscontrollers
// resources. Portainer takes only the class name and the availability; it
// fills name, type, used and new in itself, so Read keeps those as they are
// in state.
func ingressControllersBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"availability": schema.BoolAttribute{Required: true, Description: "Whether the ingress controller is exposed for selection in the Portainer UI."},
				"class_name":   schema.StringAttribute{Required: true, Description: "Kubernetes IngressClass name associated with the controller. Must exist in the cluster."},
				"name":         schema.StringAttribute{Required: true, Description: "Display name of the ingress controller. Informational: Portainer ignores it."},
				"new":          schema.BoolAttribute{Required: true, Description: "Whether Portainer treats the controller as newly registered. Informational: Portainer ignores it."},
				"type":         schema.StringAttribute{Required: true, Description: "Type of the ingress controller (for example `nginx`, `traefik`, `custom`). Informational: Portainer ignores it."},
				"used":         schema.BoolAttribute{Required: true, Description: "Whether the ingress controller is in use by workloads. Informational: Portainer ignores it."},
			},
		},
	}
}

// ingressControllersFromData builds the API list from the controllers of the
// SDKv2 view; available forces every controller available (destroy).
func ingressControllersFromData(data map[string]interface{}, forceAvailable bool) []IngressController {
	raw := k8sfwList(data, "controllers")
	controllers := make([]IngressController, 0, len(raw))
	for _, item := range raw {
		c, _ := item.(map[string]interface{})
		availability, _ := c["availability"].(bool)
		if forceAvailable {
			availability = true
		}
		name, _ := c["name"].(string)
		className, _ := c["class_name"].(string)
		typ, _ := c["type"].(string)
		used, _ := c["used"].(bool)
		isNew, _ := c["new"].(bool)
		controllers = append(controllers, IngressController{
			Availability: availability,
			ClassName:    className,
			Name:         name,
			New:          isNew,
			Type:         typ,
			Used:         used,
		})
	}
	return controllers
}

// refreshIngressControllers returns the configured controllers, in their
// order, with the availability the cluster reports. Portainer lists every
// controller of the cluster and fills name, type, used and new in itself, and
// takes nothing but the class name and the availability on update, so reading
// the list back verbatim planned changes (or, per namespace, a replacement)
// whenever the cluster had other controllers or an ingress started using one.
// A configured class the cluster no longer has drops out, which shows as
// drift. An import, with nothing configured, takes every controller.
func refreshIngressControllers(actual []IngressController, configured []interface{}) []map[string]interface{} {
	byClass := make(map[string]IngressController, len(actual))
	for _, c := range actual {
		byClass[c.ClassName] = c
	}
	if len(configured) == 0 {
		list := make([]map[string]interface{}, 0, len(actual))
		for _, c := range actual {
			list = append(list, map[string]interface{}{
				"availability": c.Availability,
				"class_name":   c.ClassName,
				"name":         c.Name,
				"new":          c.New,
				"type":         c.Type,
				"used":         c.Used,
			})
		}
		return list
	}
	list := make([]map[string]interface{}, 0, len(configured))
	for _, raw := range configured {
		want := raw.(map[string]interface{})
		c, ok := byClass[want["class_name"].(string)]
		if !ok {
			continue
		}
		list = append(list, map[string]interface{}{
			"availability": c.Availability,
			"class_name":   c.ClassName,
			"name":         want["name"],
			"new":          want["new"],
			"type":         want["type"],
			"used":         want["used"],
		})
	}
	return list
}

// checkIngressClassesExist fails when a configured class is not in the
// cluster: Portainer ignores it on update, so the apply would succeed and
// every following plan would add it again. It runs before the update, so a
// rejected list changes nothing.
func checkIngressClassesExist(ctx context.Context, client *APIClient, url string, configured []IngressController) error {
	var actual []IngressController
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &actual); err != nil {
		return fmt.Errorf("failed to read ingress controllers: %w", err)
	}
	available := make([]string, 0, len(actual))
	for _, c := range actual {
		available = append(available, c.ClassName)
	}
	for _, c := range configured {
		if !slices.Contains(available, c.ClassName) {
			return fmt.Errorf("ingress class %q is not in the cluster; available classes: %s", c.ClassName, strings.Join(available, ", "))
		}
	}
	return nil
}

// putIngressControllers checks and stores the configured controllers at url.
func putIngressControllers(ctx context.Context, client *APIClient, envID int, url string, controllers []IngressController, what string) error {
	if err := checkIngressClassesExist(ctx, client, url, controllers); err != nil {
		return err
	}
	unlock := lockKubernetesConfig(envID)
	err := doJSON(ctx, client, http.MethodPut, url, controllers, nil)
	unlock()
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", what, err)
	}
	return nil
}

// readIngressControllers refreshes the controllers of data from url,
// reporting false when the environment (or namespace) is gone.
func readIngressControllers(ctx context.Context, client *APIClient, url string, data map[string]interface{}, what string) (bool, error) {
	var controllers []IngressController
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &controllers); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read %s: %w", what, err)
	}
	data["controllers"] = refreshIngressControllers(controllers, k8sfwList(data, "controllers"))
	return true, nil
}

func (r *kubernetesIngressControllersResource) url(data map[string]interface{}) (int, string) {
	id, _ := data["environment_id"].(int)
	return id, fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", r.client.Endpoint, id)
}

func (r *kubernetesIngressControllersResource) write(ctx context.Context, data map[string]interface{}, diags *diag.Diagnostics) {
	id, url := r.url(data)
	if err := putIngressControllers(ctx, r.client, id, url, ingressControllersFromData(data, false), "ingress controllers"); err != nil {
		diags.AddError("Failed to update the ingress controllers", err.Error())
		return
	}
	data["id"] = strconv.Itoa(id)
	if _, err := readIngressControllers(ctx, r.client, url, data, "ingress controllers"); err != nil {
		diags.AddError("Failed to read the ingress controllers", err.Error())
	}
}

func (r *kubernetesIngressControllersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesIngressControllersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, url := r.url(data)
	found, err := readIngressControllers(ctx, r.client, url, data, "ingress controllers")
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the ingress controllers", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesIngressControllersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete makes the controllers available again, which is Portainer's
// default: there is no DELETE endpoint, and disabling them blocked the
// classes for the whole cluster once the resource was gone.
func (r *kubernetesIngressControllersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	data, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, url := r.url(data)
	unlock := lockKubernetesConfig(id)
	defer unlock()
	if err := doJSON(ctx, r.client, http.MethodPut, url, ingressControllersFromData(data, true), nil); err != nil {
		resp.Diagnostics.AddError("Failed to restore the ingress controllers", fmt.Errorf("failed to restore ingress controllers: %w", err).Error())
	}
}

// ImportState takes the environment ID.
func (r *kubernetesIngressControllersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected <environment_id>", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), int64(id))...)
}
