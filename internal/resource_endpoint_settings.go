package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
	AllowVolumeBrowserForRegularUsers         bool `json:"allowVolumeBrowserForRegularUsers"`
	EnableHostManagementFeatures              bool `json:"enableHostManagementFeatures"`
}

type EndpointSettingsPayload struct {
	EnableGPUManagement     bool               `json:"enableGPUManagement"`
	EnableImageNotification bool               `json:"enableImageNotification,omitempty"`
	GPUs                    []GPU              `json:"gpus,omitempty"`
	ChangeWindow            *ChangeWindow      `json:"changeWindow,omitempty"`
	DeploymentOptions       *DeploymentOptions `json:"deploymentOptions,omitempty"`
	SecuritySettings        *SecuritySettings  `json:"securitySettings,omitempty"`
}

func resourceEndpointSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointSettingsUpdate,
		Read:   resourceEndpointSettingsRead,
		Update: resourceEndpointSettingsUpdate,
		Delete: resourceEndpointSettingsDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id":               {Type: schema.TypeInt, Required: true, ForceNew: true, Description: "ID of the Portainer environment whose runtime settings are managed."},
			"enable_gpu_management":     {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether GPU management is enabled for the Portainer environment."},
			"enable_image_notification": {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether image update notifications are enabled for the Portainer environment."},
			"gpus": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of GPU devices exposed to Portainer for this environment.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":  {Type: schema.TypeString, Required: true, Description: "Display name of the GPU device."},
						"value": {Type: schema.TypeString, Required: true, Description: "Device identifier of the GPU as exposed to Docker."},
					},
				},
			},
			"change_window": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Maintenance change window during which automatic updates may run on the environment.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled":    {Type: schema.TypeBool, Optional: true, Description: "Whether the change window is enabled."},
						"start_time": {Type: schema.TypeString, Optional: true, Description: "Start time of the change window in HH:MM format."},
						"end_time":   {Type: schema.TypeString, Optional: true, Description: "End time of the change window in HH:MM format."},
					},
				},
			},
			"deployment_options": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Per-environment overrides for stack and application deployment UI options.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hide_add_with_form":      {Type: schema.TypeBool, Optional: true, Description: "Whether to hide the form-based add option in the deployment UI."},
						"hide_file_upload":        {Type: schema.TypeBool, Optional: true, Description: "Whether to hide the file upload option in the deployment UI."},
						"hide_web_editor":         {Type: schema.TypeBool, Optional: true, Description: "Whether to hide the web editor option in the deployment UI."},
						"override_global_options": {Type: schema.TypeBool, Optional: true, Description: "Whether to override global deployment options with these environment-specific values."},
					},
				},
			},
			"security_settings": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Per-environment security settings controlling what regular (non-admin) users may do.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"allow_bind_mounts":            {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may use bind mounts when deploying containers."},
						"allow_container_capabilities": {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may add Linux capabilities to containers."},
						"allow_device_mapping":         {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may map host devices into containers."},
						"allow_host_namespace":         {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may use the host namespace for containers."},
						"allow_privileged_mode":        {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may run containers in privileged mode."},
						"allow_stack_management":       {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may create and manage stacks."},
						"allow_sysctl_setting":         {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may set sysctl values on containers."},
						"allow_volume_browser":         {Type: schema.TypeBool, Optional: true, Description: "Whether regular users may browse the contents of volumes via Portainer."},
						"enable_host_management":       {Type: schema.TypeBool, Optional: true, Description: "Whether host management features (host browser, host info) are enabled in the UI."},
					},
				},
			},
		},
	}
}

func resourceEndpointSettingsUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	// Parse security settings
	var sec *SecuritySettings
	if v, ok := d.GetOk("security_settings"); ok {
		m := v.([]interface{})[0].(map[string]interface{})
		sec = &SecuritySettings{
			AllowBindMountsForRegularUsers:            m["allow_bind_mounts"].(bool),
			AllowContainerCapabilitiesForRegularUsers: m["allow_container_capabilities"].(bool),
			AllowDeviceMappingForRegularUsers:         m["allow_device_mapping"].(bool),
			AllowHostNamespaceForRegularUsers:         m["allow_host_namespace"].(bool),
			AllowPrivilegedModeForRegularUsers:        m["allow_privileged_mode"].(bool),
			AllowStackManagementForRegularUsers:       m["allow_stack_management"].(bool),
			AllowSysctlSettingForRegularUsers:         m["allow_sysctl_setting"].(bool),
			AllowVolumeBrowserForRegularUsers:         m["allow_volume_browser"].(bool),
			EnableHostManagementFeatures:              m["enable_host_management"].(bool),
		}
	}

	// Parse change window
	var cw *ChangeWindow
	if v, ok := d.GetOk("change_window"); ok {
		m := v.([]interface{})[0].(map[string]interface{})
		cw = &ChangeWindow{
			Enabled:   m["enabled"].(bool),
			StartTime: m["start_time"].(string),
			EndTime:   m["end_time"].(string),
		}
	}

	// Parse deployment options
	var deploy *DeploymentOptions
	if v, ok := d.GetOk("deployment_options"); ok {
		m := v.([]interface{})[0].(map[string]interface{})
		deploy = &DeploymentOptions{
			HideAddWithForm:       m["hide_add_with_form"].(bool),
			HideFileUpload:        m["hide_file_upload"].(bool),
			HideWebEditor:         m["hide_web_editor"].(bool),
			OverrideGlobalOptions: m["override_global_options"].(bool),
		}
	}

	// Parse GPUs
	var gpus []GPU
	if v, ok := d.GetOk("gpus"); ok {
		for _, raw := range v.([]interface{}) {
			g := raw.(map[string]interface{})
			gpus = append(gpus, GPU{
				Name:  g["name"].(string),
				Value: g["value"].(string),
			})
		}
	}

	payload := EndpointSettingsPayload{
		EnableGPUManagement:     d.Get("enable_gpu_management").(bool),
		EnableImageNotification: d.Get("enable_image_notification").(bool),
		GPUs:                    gpus,
		SecuritySettings:        sec,
		ChangeWindow:            cw,
		DeploymentOptions:       deploy,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/endpoints/%d/settings", client.Endpoint, endpointID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update endpoint settings: %s", string(data))
	}
	d.SetId(strconv.Itoa(endpointID))
	return nil
}

func resourceEndpointSettingsRead(d *schema.ResourceData, meta interface{}) error {
	d.SetId(strconv.Itoa(d.Get("endpoint_id").(int)))
	return nil
}

func resourceEndpointSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
