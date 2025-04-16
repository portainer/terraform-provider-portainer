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

type EndpointSettingsPayload struct {
	AllowBindMountsForRegularUsers            bool  `json:"allowBindMountsForRegularUsers"`
	AllowContainerCapabilitiesForRegularUsers bool  `json:"allowContainerCapabilitiesForRegularUsers"`
	AllowDeviceMappingForRegularUsers         bool  `json:"allowDeviceMappingForRegularUsers"`
	AllowHostNamespaceForRegularUsers         bool  `json:"allowHostNamespaceForRegularUsers"`
	AllowPrivilegedModeForRegularUsers        bool  `json:"allowPrivilegedModeForRegularUsers"`
	AllowStackManagementForRegularUsers       bool  `json:"allowStackManagementForRegularUsers"`
	AllowSysctlSettingForRegularUsers         bool  `json:"allowSysctlSettingForRegularUsers"`
	AllowVolumeBrowserForRegularUsers         bool  `json:"allowVolumeBrowserForRegularUsers"`
	EnableGPUManagement                       bool  `json:"enableGPUManagement"`
	EnableHostManagementFeatures              bool  `json:"enableHostManagementFeatures"`
	GPUs                                      []GPU `json:"gpus"`
}

func resourceEndpointSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointSettingsUpdate,
		Read:   resourceEndpointSettingsRead,
		Update: resourceEndpointSettingsUpdate,
		Delete: resourceEndpointSettingsDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"allow_bind_mounts": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"allow_container_capabilities": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_device_mapping": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_host_namespace": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_privileged_mode": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"allow_stack_management": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_sysctl_setting": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"allow_volume_browser": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"enable_gpu_management": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"enable_host_management": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"gpus": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceEndpointSettingsUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)

	payload := EndpointSettingsPayload{
		AllowBindMountsForRegularUsers:            d.Get("allow_bind_mounts").(bool),
		AllowContainerCapabilitiesForRegularUsers: d.Get("allow_container_capabilities").(bool),
		AllowDeviceMappingForRegularUsers:         d.Get("allow_device_mapping").(bool),
		AllowHostNamespaceForRegularUsers:         d.Get("allow_host_namespace").(bool),
		AllowPrivilegedModeForRegularUsers:        d.Get("allow_privileged_mode").(bool),
		AllowStackManagementForRegularUsers:       d.Get("allow_stack_management").(bool),
		AllowSysctlSettingForRegularUsers:         d.Get("allow_sysctl_setting").(bool),
		AllowVolumeBrowserForRegularUsers:         d.Get("allow_volume_browser").(bool),
		EnableGPUManagement:                       d.Get("enable_gpu_management").(bool),
		EnableHostManagementFeatures:              d.Get("enable_host_management").(bool),
	}

	if v, ok := d.GetOk("gpus"); ok {
		gpuList := v.([]interface{})
		for _, g := range gpuList {
			gpu := g.(map[string]interface{})
			payload.GPUs = append(payload.GPUs, GPU{
				Name:  gpu["name"].(string),
				Value: gpu["value"].(string),
			})
		}
	}

	jsonBody, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/endpoints/%d/settings", client.Endpoint, endpointID)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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
