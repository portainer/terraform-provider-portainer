package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesPersistentVolumeResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesPersistentVolumeResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesPersistentVolumeResource)(nil)
)

func newKubernetesPersistentVolumeResource() resource.Resource {
	return &kubernetesPersistentVolumeResource{}
}

type kubernetesPersistentVolumeResource struct{ k8sfwBase }

type kubernetesPersistentVolumeModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`
	ReclaimPolicy types.String `tfsdk:"reclaim_policy"`
	Phase         types.String `tfsdk:"phase"`
	Capacity      types.String `tfsdk:"capacity"`
	StorageClass  types.String `tfsdk:"storage_class"`
	AccessModes   types.List   `tfsdk:"access_modes"`
	ClaimRef      types.String `tfsdk:"claim_ref"`
}

func (r *kubernetesPersistentVolumeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_persistent_volume"
}

func (r *kubernetesPersistentVolumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the volume, `<environment_id>:<name>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment the volume belongs to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of an existing PersistentVolume. Portainer has no API to create one, so this resource adopts a volume created by a manifest, a StorageClass provisioner or a Helm chart.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"reclaim_policy": schema.StringAttribute{
				Required:    true,
				Description: "Reclaim policy applied to the volume: `Retain`, `Delete` or `Recycle`. This is the one property of a bound PersistentVolume Kubernetes allows to be changed in place.",
				Validators:  []validator.String{stringvalidator.OneOf("Retain", "Delete", "Recycle")},
			},
			"phase": schema.StringAttribute{
				Computed:    true,
				Description: "Lifecycle phase of the volume: `Available`, `Bound`, `Released`, `Failed` or `Pending`.",
			},
			"capacity": schema.StringAttribute{
				Computed:    true,
				Description: "Storage capacity of the volume as a Kubernetes quantity string (for example `10Gi`).",
			},
			"storage_class": schema.StringAttribute{
				Computed:    true,
				Description: "StorageClass the volume belongs to, empty for a statically provisioned volume.",
			},
			"access_modes": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Access modes the volume supports, such as `ReadWriteOnce` or `ReadWriteMany`.",
			},
			"claim_ref": schema.StringAttribute{
				Computed:    true,
				Description: "PersistentVolumeClaim bound to this volume as `<namespace>/<name>`, empty while the volume is unbound.",
			},
		},
	}
}

func (r *kubernetesPersistentVolumeResource) apply(ctx context.Context, plan *kubernetesPersistentVolumeModel) (bool, error) {
	envID := plan.EnvironmentID.ValueInt64()
	name := plan.Name.ValueString()

	payload := map[string]interface{}{
		"name":          name,
		"reclaimPolicy": plan.ReclaimPolicy.ValueString(),
	}
	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/reclaim_policy", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPut, url, payload, nil); err != nil {
		return false, fmt.Errorf("failed to set the reclaim policy of persistent volume %q: %w", name, err)
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d:%s", envID, name))
	// The configured policy is what the apply stores; a refresh reports any
	// difference.
	policy := plan.ReclaimPolicy
	found, err := r.read(ctx, plan)
	plan.ReclaimPolicy = policy
	return found, err
}

// read refreshes m from the volume, reporting false when it is gone.
func (r *kubernetesPersistentVolumeResource) read(ctx context.Context, m *kubernetesPersistentVolumeModel) (bool, error) {
	envID := m.EnvironmentID.ValueInt64()
	name := m.Name.ValueString()

	var pv struct {
		Spec struct {
			Capacity                      map[string]string `json:"capacity"`
			AccessModes                   []string          `json:"accessModes"`
			PersistentVolumeReclaimPolicy string            `json:"persistentVolumeReclaimPolicy"`
			StorageClassName              string            `json:"storageClassName"`
			ClaimRef                      struct {
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"claimRef"`
		} `json:"spec"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/%s", r.client.Endpoint, envID, name)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &pv); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read persistent volume %q: %w", name, err)
	}

	claimRef := ""
	if pv.Spec.ClaimRef.Name != "" {
		claimRef = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
	}
	accessModes, _ := types.ListValueFrom(ctx, types.StringType, append([]string{}, pv.Spec.AccessModes...))

	m.ReclaimPolicy = types.StringValue(pv.Spec.PersistentVolumeReclaimPolicy)
	m.Phase = types.StringValue(pv.Status.Phase)
	m.Capacity = types.StringValue(pv.Spec.Capacity["storage"])
	m.StorageClass = types.StringValue(pv.Spec.StorageClassName)
	m.AccessModes = accessModes
	m.ClaimRef = types.StringValue(claimRef)
	return true, nil
}

func (r *kubernetesPersistentVolumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesPersistentVolumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.apply(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to adopt the persistent volume", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to adopt the persistent volume", fmt.Sprintf("persistent volume %q not found", plan.Name.ValueString()))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *kubernetesPersistentVolumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesPersistentVolumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the persistent volume", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesPersistentVolumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesPersistentVolumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.apply(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update the persistent volume", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to update the persistent volume", fmt.Sprintf("persistent volume %q not found", plan.Name.ValueString()))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *kubernetesPersistentVolumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesPersistentVolumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := state.EnvironmentID.ValueInt64()
	name := state.Name.ValueString()

	// The delete endpoint takes a list, so a single volume is sent as a
	// one-element array.
	url := fmt.Sprintf("%s/kubernetes/%d/persistent_volumes/delete", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, []string{name}, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete the persistent volume", fmt.Errorf("failed to delete persistent volume %q: %w", name, err).Error())
	}
}

// ImportState takes "<environment_id>:<name>", or the "<environment_id>/<name>"
// form from before v3.0.0, and stores the ID in the current format.
func (r *kubernetesPersistentVolumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	const format = "<environment_id>:<name>"
	parts, err := importIDPartsCompat(req.ID, "/", 2, format)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	envID, err := strconv.Atoi(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected %s", req.ID, format))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strings.Join(parts, ":"))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), int64(envID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}
