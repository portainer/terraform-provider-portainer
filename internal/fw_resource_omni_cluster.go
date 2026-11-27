package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*omniClusterResource)(nil)
	_ resource.ResourceWithConfigure   = (*omniClusterResource)(nil)
	_ resource.ResourceWithImportState = (*omniClusterResource)(nil)
)

// omniClusterResource manages a Talos cluster provisioned through Siderolabs
// Omni. Creating one also creates the Portainer environment that fronts it,
// which is why endpoint_id is reported back.
//
// Portainer's read only returns the cluster's spec and status, never the
// machines it was built from, so the control plane and worker blocks are
// configuration-only: they are never read back, and drift in the machine list
// is not something this resource can detect.
func newOmniClusterResource() resource.Resource { return &omniClusterResource{} }

type omniClusterResource struct{ client *APIClient }

type omniClusterModel struct {
	ID                       types.String `tfsdk:"id"`
	CredentialID             types.Int64  `tfsdk:"credential_id"`
	Name                     types.String `tfsdk:"name"`
	Kind                     types.String `tfsdk:"kind"`
	KubernetesVersion        types.String `tfsdk:"kubernetes_version"`
	TalosVersion             types.String `tfsdk:"talos_version"`
	Labels                   types.Map    `tfsdk:"labels"`
	ClusterConfig            types.String `tfsdk:"cluster_config"`
	PortainerURL             types.String `tfsdk:"portainer_url"`
	TunnelServerAddress      types.String `tfsdk:"tunnel_server_address"`
	ValidateBeforeCreate     types.Bool   `tfsdk:"validate_before_create"`
	ClusterPatch             types.List   `tfsdk:"cluster_patch"`
	ControlPlane             types.List   `tfsdk:"control_plane"`
	Worker                   types.List   `tfsdk:"worker"`
	EndpointID               types.Int64  `tfsdk:"endpoint_id"`
	Phase                    types.Int64  `tfsdk:"phase"`
	Ready                    types.Bool   `tfsdk:"ready"`
	Available                types.Bool   `tfsdk:"available"`
	ControlPlaneReady        types.Bool   `tfsdk:"control_plane_ready"`
	KubernetesAPIReady       types.Bool   `tfsdk:"kubernetes_api_ready"`
	MachinesTotal            types.Int64  `tfsdk:"machines_total"`
	MachinesRequested        types.Int64  `tfsdk:"machines_requested"`
	MachinesConnected        types.Int64  `tfsdk:"machines_connected"`
	MachinesHealthy          types.Int64  `tfsdk:"machines_healthy"`
	DiskEncryption           types.Bool   `tfsdk:"disk_encryption"`
	WorkloadProxyEnabled     types.Bool   `tfsdk:"workload_proxy_enabled"`
	EmbeddedDiscoveryService types.Bool   `tfsdk:"embedded_discovery_service"`
	BackupEnabled            types.Bool   `tfsdk:"backup_enabled"`
	BackupInterval           types.String `tfsdk:"backup_interval"`
}

func (r *omniClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_cluster"
}

// Optional arguments inside the blocks: see omniSDKZeroIsNull.
func omniBlockString(description string, validators ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   description,
		Validators:    validators,
		PlanModifiers: []planmodifier.String{omniSDKZeroIsNull{}},
	}
}

func omniBlockInt(description string) schema.Int64Attribute {
	return schema.Int64Attribute{
		Optional:      true,
		Computed:      true,
		Description:   description,
		PlanModifiers: []planmodifier.Int64{omniSDKZeroIsNull{}},
	}
}

func omniBlockStrings(description string) schema.ListAttribute {
	return schema.ListAttribute{
		Optional:      true,
		Computed:      true,
		ElementType:   types.StringType,
		Description:   description,
		PlanModifiers: []planmodifier.List{omniSDKZeroIsNull{}},
	}
}

func omniMachineSchemaBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					Required:    true,
					Description: "Name of the machine as Omni knows it.",
				},
				"hostname":         omniBlockString("Hostname to configure on the machine."),
				"kind":             omniBlockString("Omni resource kind of the machine. Leave unset unless Omni asks for a specific one."),
				"install_disk":     omniBlockString("Block device Talos is installed onto, for example `/dev/sda`."),
				"nameservers":      omniBlockStrings("DNS servers to configure on the machine."),
				"system_disk_size": omniBlockInt("Size of the ephemeral system volume in GiB. Omni turns this into a volume configuration patch."),
			},
			Blocks: map[string]schema.Block{
				"user_disk": schema.ListNestedBlock{
					Description: "A user volume to carve out of the remaining disk space.",
					Validators:  []validator.List{listvalidator.SizeAtMost(1)},
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"volume_name": schema.StringAttribute{
								Required:    true,
								Description: "Name of the volume.",
							},
							"size": omniBlockInt("Size of the volume in GiB. Zero means take all the space that is left."),
						},
					},
				},
				"interface": schema.ListNestedBlock{
					Description: "Network interfaces to configure on the machine.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"interface": schema.StringAttribute{
								Required:    true,
								Description: "Name of the interface, for example `eth0`.",
							},
							"dhcp": schema.BoolAttribute{
								Optional:      true,
								Computed:      true,
								Description:   "Whether the interface takes its address from DHCP. Leave `addresses` unset when it does.",
								PlanModifiers: []planmodifier.Bool{omniSDKZeroIsNull{}},
							},
							"addresses": omniBlockStrings("Static addresses in CIDR notation, for example `10.0.0.10/24`."),
						},
						Blocks: map[string]schema.Block{
							"route": schema.ListNestedBlock{
								Description: "Static routes to add through the interface.",
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"network": schema.StringAttribute{
											Required:    true,
											Description: "Destination network in CIDR notation. Use `0.0.0.0/0` for a default route.",
										},
										"gateway": schema.StringAttribute{
											Required:    true,
											Description: "Gateway the route goes through.",
										},
									},
								},
							},
						},
					},
				},
				"patch": schema.ListNestedBlock{
					Description: "Talos machine configuration patches to apply.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"id_override": omniBlockString("Identifier Omni stores the patch under, instead of the generated one."),
							"inline":      omniBlockString("Patch body as a JSON object. Use `jsonencode({...})`.", omniJSONObjectValidator{}),
						},
					},
				},
			},
		},
	}
}

func (r *omniClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedInt := func(d string) schema.Int64Attribute { return schema.Int64Attribute{Computed: true, Description: d} }
	computedBool := func(d string) schema.BoolAttribute { return schema.BoolAttribute{Computed: true, Description: d} }

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the cluster. Omni cannot rename a cluster, so changing it replaces the resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"kind": schema.StringAttribute{
				Optional:    true,
				Description: "Omni resource kind of the cluster. Leave unset unless Omni asks for a specific one.",
			},
			// Unset, the value Portainer reports is kept, and sent on update
			// as SDKv2 did.
			"kubernetes_version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Kubernetes version to run. Leave unset to let Omni pick, and read the result back from this attribute.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"talos_version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Talos version to run. Use the `portainer_omni_talos_versions` data source to see what the credential supports.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Labels to set on the Omni cluster resource.",
			},
			// Omni accepts either a structured spec or a raw template. The raw
			// one is only read at creation, so changing it replaces.
			"cluster_config": schema.StringAttribute{
				Optional:      true,
				Description:   "Raw Omni cluster template, as an alternative to describing the cluster with the blocks below. Only read when the cluster is created.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"portainer_url": schema.StringAttribute{
				Optional:    true,
				Description: "URL the provisioned nodes reach Portainer on. Leave unset to use the instance's configured URL.",
			},
			"tunnel_server_address": schema.StringAttribute{
				Optional:    true,
				Description: "Address the provisioned nodes reach Portainer's tunnel server on.",
			},
			// Provisioning a Talos cluster touches real machines and takes a
			// long time. Portainer offers a cheap validation call, so the
			// default is to use it rather than to find out half way through.
			"validate_before_create": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to validate the cluster configuration with Portainer before provisioning it. Defaults to `true`.",
			},
			"endpoint_id": schema.Int64Attribute{
				Computed:      true,
				Description:   "Identifier of the Portainer environment created for the cluster.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"phase":                      computedInt("Provisioning phase Omni reports for the cluster."),
			"ready":                      computedBool("Whether Omni reports the cluster as ready."),
			"available":                  computedBool("Whether Omni reports the cluster as available."),
			"control_plane_ready":        computedBool("Whether the control plane has come up."),
			"kubernetes_api_ready":       computedBool("Whether the cluster's Kubernetes API is answering."),
			"machines_total":             computedInt("Machines Omni counts in the cluster."),
			"machines_requested":         computedInt("Machines the cluster asked for."),
			"machines_connected":         computedInt("Machines currently connected to Omni."),
			"machines_healthy":           computedInt("Machines Omni considers healthy."),
			"disk_encryption":            computedBool("Whether disk encryption is enabled on the cluster."),
			"workload_proxy_enabled":     computedBool("Whether Omni's workload proxy is enabled for the cluster."),
			"embedded_discovery_service": computedBool("Whether the cluster uses Omni's embedded discovery service."),
			"backup_enabled":             computedBool("Whether Omni backs the cluster's etcd up on a schedule."),
			"backup_interval": schema.StringAttribute{
				Computed:    true,
				Description: "Interval between the scheduled etcd backups.",
			},
		},
		Blocks: map[string]schema.Block{
			"cluster_patch": schema.ListNestedBlock{
				Description: "Talos configuration patches to apply to the cluster as a whole.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id_override":     omniBlockString("Identifier Omni stores the patch under, instead of the generated one."),
						"annotation_name": omniBlockString("Name annotation Omni shows for the patch."),
						"inline":          omniBlockString("Patch body as a JSON object. Use `jsonencode({...})`.", omniJSONObjectValidator{}),
					},
				},
			},
			"control_plane": schema.ListNestedBlock{
				Description: "The cluster's control plane.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"kind": omniBlockString("Omni resource kind of the control plane. Leave unset unless Omni asks for a specific one."),
					},
					Blocks: map[string]schema.Block{
						"machine": omniMachineSchemaBlock("Machines that make up the cluster's control plane."),
					},
				},
			},
			"worker": schema.ListNestedBlock{
				Description: "The cluster's worker pool.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": omniBlockString("Name of the worker pool."),
						"kind": omniBlockString("Omni resource kind of the worker pool. Leave unset unless Omni asks for a specific one."),
					},
					Blocks: map[string]schema.Block{
						"machine": omniMachineSchemaBlock("Machines that make up this worker pool."),
					},
				},
			},
		},
	}
}

func (r *omniClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *omniClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan omniClusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, _ := omniSDKValue(req.Plan.Raw).(map[string]interface{})
	credentialID := int(plan.CredentialID.ValueInt64())
	name := plan.Name.ValueString()

	cluster, err := omniClusterSpec(d)
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}
	controlPlane, hasControlPlane, err := omniControlPlane(d["control_plane"])
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}
	worker, hasWorker, err := omniWorker(d["worker"])
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}

	payload := map[string]interface{}{"cluster": cluster}
	if hasControlPlane {
		payload["controlPlane"] = controlPlane
	}
	if hasWorker {
		payload["worker"] = worker
	}
	if v := plan.ClusterConfig.ValueString(); v != "" {
		payload["clusterConfig"] = v
	}
	if v := plan.PortainerURL.ValueString(); v != "" {
		payload["portainerUrl"] = v
	}
	if v := plan.TunnelServerAddress.ValueString(); v != "" {
		payload["tunnelServerAddr"] = v
	}

	// Validation is a cheap call that catches a bad specification before any
	// machine is touched, so it runs first unless it was turned off.
	if plan.ValidateBeforeCreate.ValueBool() {
		validateURL := fmt.Sprintf("%s/omni/%d/cluster/validate", r.client.Endpoint, credentialID)
		if err := doJSON(ctx, r.client, http.MethodPost, validateURL, payload, nil); err != nil {
			omniAddError(&resp.Diagnostics, fmt.Errorf("the Omni cluster configuration was rejected before provisioning started (set validate_before_create = false to skip this check): %w", err))
			return
		}
	}

	var environment struct {
		ID int `json:"Id"`
	}
	createURL := fmt.Sprintf("%s/omni/%d/cluster/create", r.client.Endpoint, credentialID)
	if err := doJSON(ctx, r.client, http.MethodPost, createURL, payload, &environment); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to create Omni cluster %s: %w", name, err))
		return
	}

	plan.EndpointID = types.Int64Value(int64(environment.ID))
	plan.ID = types.StringValue(omniClusterID(credentialID, name))
	// Saved before the read, so a failing read still leaves the created
	// cluster in state.
	resp.Diagnostics.Append(resp.State.Set(ctx, omniClusterUnknownToNull(plan))...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.read(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError(fmt.Sprintf("Omni cluster %s was not found after it was created", name), "")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// omniClusterUnknownToNull makes a planned model storable: computed values
// still unknown become null until the read fills them.
func omniClusterUnknownToNull(m omniClusterModel) *omniClusterModel {
	str := func(v types.String) types.String {
		if v.IsUnknown() {
			return types.StringNull()
		}
		return v
	}
	i := func(v types.Int64) types.Int64 {
		if v.IsUnknown() {
			return types.Int64Null()
		}
		return v
	}
	b := func(v types.Bool) types.Bool {
		if v.IsUnknown() {
			return types.BoolNull()
		}
		return v
	}
	m.KubernetesVersion, m.TalosVersion, m.BackupInterval = str(m.KubernetesVersion), str(m.TalosVersion), str(m.BackupInterval)
	m.Phase, m.MachinesTotal, m.MachinesRequested = i(m.Phase), i(m.MachinesTotal), i(m.MachinesRequested)
	m.MachinesConnected, m.MachinesHealthy = i(m.MachinesConnected), i(m.MachinesHealthy)
	m.Ready, m.Available, m.ControlPlaneReady, m.KubernetesAPIReady = b(m.Ready), b(m.Available), b(m.ControlPlaneReady), b(m.KubernetesAPIReady)
	m.DiskEncryption, m.WorkloadProxyEnabled = b(m.DiskEncryption), b(m.WorkloadProxyEnabled)
	m.EmbeddedDiscoveryService, m.BackupEnabled = b(m.EmbeddedDiscoveryService), b(m.BackupEnabled)
	return &m
}

// read fills the model from Portainer. It returns false when the cluster is
// gone. The machine blocks are deliberately left alone: Portainer's read
// reports the cluster's spec and status but not the machines it was built
// from, so writing anything into them would be a guess.
func (r *omniClusterResource) read(ctx context.Context, m *omniClusterModel, diags *diag.Diagnostics) bool {
	credentialID, name, err := omniClusterParts(m.ID.ValueString())
	if err != nil {
		omniAddError(diags, err)
		return false
	}
	// Moves an ID from before v3.0.0 ("/") to the current format (":").
	m.ID = types.StringValue(omniClusterID(credentialID, name))

	var cluster struct {
		Metadata struct {
			KubernetesVersion string `json:"kubernetes_version"`
			TalosVersion      string `json:"talos_version"`
			Features          struct {
				DiskEncryption              bool `json:"disk_encryption"`
				EnableWorkloadProxy         bool `json:"enable_workload_proxy"`
				UseEmbeddedDiscoveryService bool `json:"use_embedded_discovery_service"`
			} `json:"features"`
			BackupConfiguration struct {
				Enabled  bool   `json:"enabled"`
				Interval string `json:"interval"`
			} `json:"backup_configuration"`
		} `json:"metadata"`
		Status struct {
			Available          bool `json:"available"`
			Ready              bool `json:"ready"`
			Phase              int  `json:"phase"`
			ControlPlaneReady  bool `json:"controlplaneReady"`
			KubernetesAPIReady bool `json:"kubernetesAPIReady"`
			Machines           struct {
				Total     int `json:"total"`
				Requested int `json:"requested"`
				Connected int `json:"connected"`
				Healthy   int `json:"healthy"`
			} `json:"machines"`
		} `json:"status"`
	}

	readURL := fmt.Sprintf("%s/omni/%d/cluster/%s", r.client.Endpoint, credentialID, url.PathEscape(name))
	if err := doJSON(ctx, r.client, http.MethodGet, readURL, nil, &cluster); err != nil {
		if isAPINotFound(err) {
			return false
		}
		omniAddError(diags, fmt.Errorf("failed to read Omni cluster %s: %w", name, err))
		return false
	}

	m.CredentialID = types.Int64Value(int64(credentialID))
	m.Name = types.StringValue(name)
	m.KubernetesVersion = types.StringValue(cluster.Metadata.KubernetesVersion)
	m.TalosVersion = types.StringValue(cluster.Metadata.TalosVersion)
	m.DiskEncryption = types.BoolValue(cluster.Metadata.Features.DiskEncryption)
	m.WorkloadProxyEnabled = types.BoolValue(cluster.Metadata.Features.EnableWorkloadProxy)
	m.EmbeddedDiscoveryService = types.BoolValue(cluster.Metadata.Features.UseEmbeddedDiscoveryService)
	m.BackupEnabled = types.BoolValue(cluster.Metadata.BackupConfiguration.Enabled)
	m.BackupInterval = types.StringValue(cluster.Metadata.BackupConfiguration.Interval)
	m.Available = types.BoolValue(cluster.Status.Available)
	m.Ready = types.BoolValue(cluster.Status.Ready)
	m.Phase = types.Int64Value(int64(cluster.Status.Phase))
	m.ControlPlaneReady = types.BoolValue(cluster.Status.ControlPlaneReady)
	m.KubernetesAPIReady = types.BoolValue(cluster.Status.KubernetesAPIReady)
	m.MachinesTotal = types.Int64Value(int64(cluster.Status.Machines.Total))
	m.MachinesRequested = types.Int64Value(int64(cluster.Status.Machines.Requested))
	m.MachinesConnected = types.Int64Value(int64(cluster.Status.Machines.Connected))
	m.MachinesHealthy = types.Int64Value(int64(cluster.Status.Machines.Healthy))
	return true
}

func (r *omniClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state omniClusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.read(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *omniClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state omniClusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID, _, err := omniClusterParts(state.ID.ValueString())
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}

	newD, _ := omniSDKValue(req.Plan.Raw).(map[string]interface{})
	oldD, _ := omniSDKValue(req.State.Raw).(map[string]interface{})

	cluster, err := omniClusterSpec(newD)
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}
	payload := map[string]interface{}{"cluster": cluster}

	addedControlPlanes, removedControlPlanes := omniGroupDelta(oldD["control_plane"], newD["control_plane"])
	if len(addedControlPlanes) > 0 {
		group, err := omniGroupFromMachines(newD["control_plane"], addedControlPlanes)
		if err != nil {
			omniAddError(&resp.Diagnostics, err)
			return
		}
		payload["controlPlanesToAdd"] = group
	}
	if len(removedControlPlanes) > 0 {
		payload["controlPlanesToRemove"] = removedControlPlanes
	}

	addedWorkers, removedWorkers := omniGroupDelta(oldD["worker"], newD["worker"])
	if len(addedWorkers) > 0 {
		group, err := omniGroupFromMachines(newD["worker"], addedWorkers)
		if err != nil {
			omniAddError(&resp.Diagnostics, err)
			return
		}
		payload["workersToAdd"] = group
	}
	if len(removedWorkers) > 0 {
		payload["workersToRemove"] = removedWorkers
	}

	updateURL := fmt.Sprintf("%s/omni/%d/cluster/update", r.client.Endpoint, credentialID)
	if err := doJSON(ctx, r.client, http.MethodPut, updateURL, payload, nil); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to update Omni cluster %s: %w", plan.Name.ValueString(), err))
		return
	}

	plan.ID = state.ID
	found := r.read(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *omniClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state omniClusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID, name, err := omniClusterParts(state.ID.ValueString())
	if err != nil {
		omniAddError(&resp.Diagnostics, err)
		return
	}

	deleteURL := fmt.Sprintf("%s/omni/%d/cluster/delete?name=%s", r.client.Endpoint, credentialID, url.QueryEscape(name))
	if err := doJSON(ctx, r.client, http.MethodDelete, deleteURL, nil, nil); err != nil && !isAPINotFound(err) {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to delete Omni cluster %s: %w", name, err))
	}
}

// ImportState takes "<credential id>:<cluster name>" (or the "/" form from
// before v3.0.0); the read checks and normalizes it.
func (r *omniClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
