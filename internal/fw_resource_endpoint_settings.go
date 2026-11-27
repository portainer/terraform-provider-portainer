package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*endpointSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*endpointSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*endpointSettingsResource)(nil)
)

func newEndpointSettingsResource() resource.Resource { return &endpointSettingsResource{} }

type endpointSettingsResource struct{ client *APIClient }

type GPU struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ChangeWindow struct {
	Enabled   bool   `json:"Enabled,omitempty"`
	StartTime string `json:"StartTime,omitempty"`
	EndTime   string `json:"EndTime,omitempty"`
}

type DeploymentOptions struct {
	HideAddWithForm       bool `json:"hideAddWithForm,omitempty"`
	HideFileUpload        bool `json:"hideFileUpload,omitempty"`
	HideWebEditor         bool `json:"hideWebEditor,omitempty"`
	OverrideGlobalOptions bool `json:"overrideGlobalOptions,omitempty"`
}

type SecuritySettings struct {
	AllowBindMountsForRegularUsers            bool `json:"allowBindMountsForRegularUsers"`
	AllowContainerCapabilitiesForRegularUsers bool `json:"allowContainerCapabilitiesForRegularUsers"`
	AllowDeviceMappingForRegularUsers         bool `json:"allowDeviceMappingForRegularUsers"`
	AllowHostNamespaceForRegularUsers         bool `json:"allowHostNamespaceForRegularUsers"`
	AllowPrivilegedModeForRegularUsers        bool `json:"allowPrivilegedModeForRegularUsers"`
	AllowStackManagementForRegularUsers       bool `json:"allowStackManagementForRegularUsers"`
	AllowSysctlSettingForRegularUsers         bool `json:"allowSysctlSettingForRegularUsers"`
	AllowSecurityOptForRegularUsers           bool `json:"allowSecurityOptForRegularUsers"`
	AllowVolumeBrowserForRegularUsers         bool `json:"allowVolumeBrowserForRegularUsers"`
	EnableHostManagementFeatures              bool `json:"enableHostManagementFeatures"`
}

type EndpointSettingsPayload struct {
	EnableGPUManagement     bool `json:"enableGPUManagement"`
	EnableImageNotification bool `json:"enableImageNotification,omitempty"`
	// GPUs is nil (sent as null, Portainer keeps its list) unless gpus is set
	// or changed; an empty list then clears the GPUs.
	GPUs              []GPU              `json:"gpus"`
	ChangeWindow      *ChangeWindow      `json:"changeWindow,omitempty"`
	DeploymentOptions *DeploymentOptions `json:"deploymentOptions,omitempty"`
	SecuritySettings  *SecuritySettings  `json:"securitySettings,omitempty"`

	// PUT /endpoints/{id}/settings reads the security switches as top-level
	// fields; the nested securitySettings object above is ignored by
	// Portainer CE.
	AllowBindMountsForRegularUsers            *bool `json:"allowBindMountsForRegularUsers,omitempty"`
	AllowContainerCapabilitiesForRegularUsers *bool `json:"allowContainerCapabilitiesForRegularUsers,omitempty"`
	AllowDeviceMappingForRegularUsers         *bool `json:"allowDeviceMappingForRegularUsers,omitempty"`
	AllowHostNamespaceForRegularUsers         *bool `json:"allowHostNamespaceForRegularUsers,omitempty"`
	AllowPrivilegedModeForRegularUsers        *bool `json:"allowPrivilegedModeForRegularUsers,omitempty"`
	AllowStackManagementForRegularUsers       *bool `json:"allowStackManagementForRegularUsers,omitempty"`
	AllowSysctlSettingForRegularUsers         *bool `json:"allowSysctlSettingForRegularUsers,omitempty"`
	AllowSecurityOptForRegularUsers           *bool `json:"allowSecurityOptForRegularUsers,omitempty"`
	AllowVolumeBrowserForRegularUsers         *bool `json:"allowVolumeBrowserForRegularUsers,omitempty"`
	EnableHostManagementFeatures              *bool `json:"enableHostManagementFeatures,omitempty"`
}

// endpointSettingsState is the part of GET /endpoints/{id} this resource
// manages. Change window, deployment options and image notifications are
// Business Edition fields the Community Edition endpoint does not return, so
// they are left as configured.
type endpointSettingsState struct {
	EnableGPUManagement bool              `json:"EnableGPUManagement"`
	Gpus                []GPU             `json:"Gpus"`
	SecuritySettings    *SecuritySettings `json:"SecuritySettings"`
}

type endpointSettingsModel struct {
	ID                      types.String                    `tfsdk:"id"`
	EndpointID              types.Int64                     `tfsdk:"endpoint_id"`
	EnableGPUManagement     types.Bool                      `tfsdk:"enable_gpu_management"`
	EnableImageNotification types.Bool                      `tfsdk:"enable_image_notification"`
	GPUs                    []endpointSettingsGPUModel      `tfsdk:"gpus"`
	ChangeWindow            []endpointSettingsWindowModel   `tfsdk:"change_window"`
	DeploymentOptions       []endpointSettingsDeployModel   `tfsdk:"deployment_options"`
	SecuritySettings        []endpointSettingsSecurityModel `tfsdk:"security_settings"`
}

type endpointSettingsGPUModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type endpointSettingsWindowModel struct {
	Enabled   types.Bool   `tfsdk:"enabled"`
	StartTime types.String `tfsdk:"start_time"`
	EndTime   types.String `tfsdk:"end_time"`
}

type endpointSettingsDeployModel struct {
	HideAddWithForm       types.Bool `tfsdk:"hide_add_with_form"`
	HideFileUpload        types.Bool `tfsdk:"hide_file_upload"`
	HideWebEditor         types.Bool `tfsdk:"hide_web_editor"`
	OverrideGlobalOptions types.Bool `tfsdk:"override_global_options"`
}

type endpointSettingsSecurityModel struct {
	AllowBindMounts            types.Bool `tfsdk:"allow_bind_mounts"`
	AllowContainerCapabilities types.Bool `tfsdk:"allow_container_capabilities"`
	AllowDeviceMapping         types.Bool `tfsdk:"allow_device_mapping"`
	AllowHostNamespace         types.Bool `tfsdk:"allow_host_namespace"`
	AllowPrivilegedMode        types.Bool `tfsdk:"allow_privileged_mode"`
	AllowStackManagement       types.Bool `tfsdk:"allow_stack_management"`
	AllowSysctlSetting         types.Bool `tfsdk:"allow_sysctl_setting"`
	AllowSecurityOpt           types.Bool `tfsdk:"allow_security_opt"`
	AllowVolumeBrowser         types.Bool `tfsdk:"allow_volume_browser"`
	EnableHostManagement       types.Bool `tfsdk:"enable_host_management"`
}

func (r *endpointSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_settings"
}

// The primitives inside the blocks default to their zero value: SDKv2 stored
// false/"" for an unset one and treated that as unset, so existing states
// carry zeros and a removed value has to plan back to false/"".
func endpointSettingsBlockBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: description}
}

func endpointSettingsBlockString(description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: description}
}

func (r *endpointSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "ID of the Portainer environment whose runtime settings are managed.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enable_gpu_management": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Whether GPU management is enabled for the Portainer environment.",
			},
			"enable_image_notification": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Whether image update notifications are enabled for the Portainer environment.",
			},
		},
		Blocks: map[string]schema.Block{
			"gpus": schema.ListNestedBlock{
				Description: "List of GPU devices exposed to Portainer for this environment.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name":  schema.StringAttribute{Required: true, Description: "Display name of the GPU device."},
						"value": schema.StringAttribute{Required: true, Description: "Device identifier of the GPU as exposed to Docker."},
					},
				},
			},
			"change_window": schema.ListNestedBlock{
				Description: "Maintenance change window during which automatic updates may run on the environment.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"enabled":    endpointSettingsBlockBool("Whether the change window is enabled."),
						"start_time": endpointSettingsBlockString("Start time of the change window in HH:MM format."),
						"end_time":   endpointSettingsBlockString("End time of the change window in HH:MM format."),
					},
				},
			},
			"deployment_options": schema.ListNestedBlock{
				Description: "Per-environment overrides for stack and application deployment UI options.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"hide_add_with_form":      endpointSettingsBlockBool("Whether to hide the form-based add option in the deployment UI."),
						"hide_file_upload":        endpointSettingsBlockBool("Whether to hide the file upload option in the deployment UI."),
						"hide_web_editor":         endpointSettingsBlockBool("Whether to hide the web editor option in the deployment UI."),
						"override_global_options": endpointSettingsBlockBool("Whether to override global deployment options with these environment-specific values."),
					},
				},
			},
			"security_settings": schema.ListNestedBlock{
				Description: "Per-environment security settings controlling what regular (non-admin) users may do.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"allow_bind_mounts":            endpointSettingsBlockBool("Whether regular users may use bind mounts when deploying containers."),
						"allow_container_capabilities": endpointSettingsBlockBool("Whether regular users may add Linux capabilities to containers."),
						"allow_device_mapping":         endpointSettingsBlockBool("Whether regular users may map host devices into containers."),
						"allow_host_namespace":         endpointSettingsBlockBool("Whether regular users may use the host namespace for containers."),
						"allow_privileged_mode":        endpointSettingsBlockBool("Whether regular users may run containers in privileged mode."),
						"allow_stack_management":       endpointSettingsBlockBool("Whether regular users may create and manage stacks."),
						"allow_sysctl_setting":         endpointSettingsBlockBool("Whether regular users may set sysctl values on containers."),
						"allow_security_opt":           endpointSettingsBlockBool("Whether regular users may set security options (`--security-opt`) on containers."),
						"allow_volume_browser":         endpointSettingsBlockBool("Whether regular users may browse the contents of volumes via Portainer."),
						"enable_host_management":       endpointSettingsBlockBool("Whether host management features (host browser, host info) are enabled in the UI."),
					},
				},
			},
		},
	}
}

func (r *endpointSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.read(ctx, &plan, false, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	} else if !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Failed to update endpoint settings", fmt.Sprintf("environment %d not found", plan.EndpointID.ValueInt64()))
	}
}

func (r *endpointSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state endpointSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.read(ctx, &plan, false, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	} else if !resp.Diagnostics.HasError() {
		resp.State.RemoveResource(ctx)
	}
}

// apply sends PUT /endpoints/{id}/settings. prior is nil on create.
func (r *endpointSettingsResource) apply(ctx context.Context, plan, prior *endpointSettingsModel, diags *diag.Diagnostics) {
	endpointID := plan.EndpointID.ValueInt64()

	var sec *SecuritySettings
	if len(plan.SecuritySettings) > 0 {
		m := plan.SecuritySettings[0]
		sec = &SecuritySettings{
			AllowBindMountsForRegularUsers:            m.AllowBindMounts.ValueBool(),
			AllowContainerCapabilitiesForRegularUsers: m.AllowContainerCapabilities.ValueBool(),
			AllowDeviceMappingForRegularUsers:         m.AllowDeviceMapping.ValueBool(),
			AllowHostNamespaceForRegularUsers:         m.AllowHostNamespace.ValueBool(),
			AllowPrivilegedModeForRegularUsers:        m.AllowPrivilegedMode.ValueBool(),
			AllowStackManagementForRegularUsers:       m.AllowStackManagement.ValueBool(),
			AllowSysctlSettingForRegularUsers:         m.AllowSysctlSetting.ValueBool(),
			AllowSecurityOptForRegularUsers:           m.AllowSecurityOpt.ValueBool(),
			AllowVolumeBrowserForRegularUsers:         m.AllowVolumeBrowser.ValueBool(),
			EnableHostManagementFeatures:              m.EnableHostManagement.ValueBool(),
		}
	}

	var cw *ChangeWindow
	if len(plan.ChangeWindow) > 0 {
		m := plan.ChangeWindow[0]
		cw = &ChangeWindow{Enabled: m.Enabled.ValueBool(), StartTime: m.StartTime.ValueString(), EndTime: m.EndTime.ValueString()}
	}

	var deploy *DeploymentOptions
	if len(plan.DeploymentOptions) > 0 {
		m := plan.DeploymentOptions[0]
		deploy = &DeploymentOptions{
			HideAddWithForm:       m.HideAddWithForm.ValueBool(),
			HideFileUpload:        m.HideFileUpload.ValueBool(),
			HideWebEditor:         m.HideWebEditor.ValueBool(),
			OverrideGlobalOptions: m.OverrideGlobalOptions.ValueBool(),
		}
	}

	// nil (sent as null) when the configuration has no GPU.
	var gpus []GPU
	if len(plan.GPUs) > 0 {
		gpus = make([]GPU, len(plan.GPUs))
		for i, g := range plan.GPUs {
			gpus[i] = GPU{Name: g.Name.ValueString(), Value: g.Value.ValueString()}
		}
	}
	// Removing every GPU has to reach Portainer as an empty list; omitempty
	// would drop it and the GPUs would stay, planned for removal forever.
	if gpus == nil && prior != nil && len(prior.GPUs) > 0 {
		gpus = []GPU{}
	}

	payload := EndpointSettingsPayload{
		EnableGPUManagement:     plan.EnableGPUManagement.ValueBool(),
		EnableImageNotification: plan.EnableImageNotification.ValueBool(),
		GPUs:                    gpus,
		SecuritySettings:        sec,
		ChangeWindow:            cw,
		DeploymentOptions:       deploy,
	}
	if sec != nil {
		payload.AllowBindMountsForRegularUsers = new(sec.AllowBindMountsForRegularUsers)
		payload.AllowContainerCapabilitiesForRegularUsers = new(sec.AllowContainerCapabilitiesForRegularUsers)
		payload.AllowDeviceMappingForRegularUsers = new(sec.AllowDeviceMappingForRegularUsers)
		payload.AllowHostNamespaceForRegularUsers = new(sec.AllowHostNamespaceForRegularUsers)
		payload.AllowPrivilegedModeForRegularUsers = new(sec.AllowPrivilegedModeForRegularUsers)
		payload.AllowStackManagementForRegularUsers = new(sec.AllowStackManagementForRegularUsers)
		payload.AllowSysctlSettingForRegularUsers = new(sec.AllowSysctlSettingForRegularUsers)
		payload.AllowSecurityOptForRegularUsers = new(sec.AllowSecurityOptForRegularUsers)
		payload.AllowVolumeBrowserForRegularUsers = new(sec.AllowVolumeBrowserForRegularUsers)
		payload.EnableHostManagementFeatures = new(sec.EnableHostManagementFeatures)
	}

	url := fmt.Sprintf("%s/endpoints/%d/settings", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodPut, url, payload, nil); err != nil {
		diags.AddError("Failed to update endpoint settings", fmt.Errorf("failed to update endpoint settings: %w", err).Error())
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(endpointID, 10))
}

// read refreshes m from GET /endpoints/{id}; false when the environment is
// gone. The GPU list and the security block are only written back when the
// configuration manages them (or on import): both are optional without being
// computed, and filling them in otherwise would plan their removal on every
// run.
func (r *endpointSettingsResource) read(ctx context.Context, m *endpointSettingsModel, importing bool, diags *diag.Diagnostics) bool {
	endpointID := m.EndpointID.ValueInt64()
	var state endpointSettingsState
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d", r.client.Endpoint, endpointID), nil, &state); err != nil {
		if isAPINotFound(err) {
			return false
		}
		diags.AddError("Failed to read endpoint settings", fmt.Errorf("failed to read settings of environment %d: %w", endpointID, err).Error())
		return false
	}

	m.ID = types.StringValue(strconv.FormatInt(endpointID, 10))
	m.EnableGPUManagement = types.BoolValue(state.EnableGPUManagement)
	if len(m.GPUs) > 0 || importing {
		gpus := make([]endpointSettingsGPUModel, 0, len(state.Gpus))
		for _, g := range state.Gpus {
			gpus = append(gpus, endpointSettingsGPUModel{Name: types.StringValue(g.Name), Value: types.StringValue(g.Value)})
		}
		m.GPUs = gpus
	}
	if (len(m.SecuritySettings) > 0 || importing) && state.SecuritySettings != nil {
		sec := state.SecuritySettings
		m.SecuritySettings = []endpointSettingsSecurityModel{{
			AllowBindMounts:            types.BoolValue(sec.AllowBindMountsForRegularUsers),
			AllowContainerCapabilities: types.BoolValue(sec.AllowContainerCapabilitiesForRegularUsers),
			AllowDeviceMapping:         types.BoolValue(sec.AllowDeviceMappingForRegularUsers),
			AllowHostNamespace:         types.BoolValue(sec.AllowHostNamespaceForRegularUsers),
			AllowPrivilegedMode:        types.BoolValue(sec.AllowPrivilegedModeForRegularUsers),
			AllowStackManagement:       types.BoolValue(sec.AllowStackManagementForRegularUsers),
			AllowSysctlSetting:         types.BoolValue(sec.AllowSysctlSettingForRegularUsers),
			AllowSecurityOpt:           types.BoolValue(sec.AllowSecurityOptForRegularUsers),
			AllowVolumeBrowser:         types.BoolValue(sec.AllowVolumeBrowserForRegularUsers),
			EnableHostManagement:       types.BoolValue(sec.EnableHostManagementFeatures),
		}}
	}
	m.normalizeBlocks()
	return true
}

// normalizeBlocks keeps absent blocks as empty lists, as Terraform holds them.
func (m *endpointSettingsModel) normalizeBlocks() {
	if m.GPUs == nil {
		m.GPUs = []endpointSettingsGPUModel{}
	}
	if m.ChangeWindow == nil {
		m.ChangeWindow = []endpointSettingsWindowModel{}
	}
	if m.DeploymentOptions == nil {
		m.DeploymentOptions = []endpointSettingsDeployModel{}
	}
	if m.SecuritySettings == nil {
		m.SecuritySettings = []endpointSettingsSecurityModel{}
	}
}

func (r *endpointSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, false, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *endpointSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState takes the environment ID.
func (r *endpointSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	endpointID, err := strconv.Atoi(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected the numeric environment ID", req.ID))
		return
	}
	m := endpointSettingsModel{
		ID:         types.StringValue(req.ID),
		EndpointID: types.Int64Value(int64(endpointID)),
		// Business Edition only and not returned by the API: take the schema
		// default, or the first plan after an import would set it.
		EnableImageNotification: types.BoolValue(false),
	}
	if !r.read(ctx, &m, true, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Import failed", fmt.Sprintf("environment %d not found", endpointID))
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
