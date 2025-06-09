package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerNetworkCreate,
		Read:   resourceDockerNetworkRead,
		Delete: resourceDockerNetworkDelete,
		Update: nil,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true},
			"driver":      {Type: schema.TypeString, Optional: true, Default: "bridge", ForceNew: true},
			"scope":       {Type: schema.TypeString, Optional: true, ForceNew: true},
			"internal":    {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"attachable":  {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"ingress":     {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"config_only": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"config_from": {Type: schema.TypeString, Optional: true, ForceNew: true},
			"enable_ipv4": {Type: schema.TypeBool, Optional: true, Default: true, ForceNew: true},
			"enable_ipv6": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"options":     {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"labels":      {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"ipam_driver": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "default",
				ForceNew: true,
			},
			"ipam_options": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"ipam_config": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnet": {
							Type:     schema.TypeString,
							Required: true,
						},
						"ip_range": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"gateway": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"auxiliary_addresses": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourceDockerNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	payload := map[string]interface{}{
		"Name":       d.Get("name").(string),
		"Driver":     d.Get("driver").(string),
		"Internal":   d.Get("internal").(bool),
		"Attachable": d.Get("attachable").(bool),
		"Ingress":    d.Get("ingress").(bool),
		"ConfigOnly": d.Get("config_only").(bool),
		"EnableIPv4": d.Get("enable_ipv4").(bool),
		"EnableIPv6": d.Get("enable_ipv6").(bool),
		"Options":    d.Get("options").(map[string]interface{}),
		"Labels":     d.Get("labels").(map[string]interface{}),
	}

	if v, ok := d.GetOk("scope"); ok {
		payload["Scope"] = v.(string)
	}
	if v, ok := d.GetOk("config_from"); ok {
		payload["ConfigFrom"] = map[string]string{"Network": v.(string)}
	}

	// IPAM config
	ipam := map[string]interface{}{
		"Driver": d.Get("ipam_driver").(string),
	}

	if v, ok := d.GetOk("ipam_options"); ok {
		ipam["Options"] = v.(map[string]interface{})
	}

	if v, ok := d.GetOk("ipam_config"); ok {
		configList := v.([]interface{})
		var ipamConfigs []map[string]interface{}
		for _, c := range configList {
			item := c.(map[string]interface{})
			config := map[string]interface{}{
				"Subnet": item["subnet"].(string),
			}
			if ipr, ok := item["ip_range"]; ok && ipr.(string) != "" {
				config["IPRange"] = ipr.(string)
			}
			if gw, ok := item["gateway"]; ok && gw.(string) != "" {
				config["Gateway"] = gw.(string)
			}
			if aux, ok := item["auxiliary_addresses"]; ok {
				config["AuxiliaryAddresses"] = aux.(map[string]interface{})
			}
			ipamConfigs = append(ipamConfigs, config)
		}
		ipam["Config"] = ipamConfigs
	}

	payload["IPAM"] = ipam

	var response struct {
		ID string `json:"Id"`
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker network: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	d.SetId(response.ID)
	return nil
}

func resourceDockerNetworkRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	networkID := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, networkID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read docker network: %s", string(body))
	}

	var network struct {
		ID         string            `json:"Id"`
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Scope      string            `json:"Scope"`
		Internal   bool              `json:"Internal"`
		Attachable bool              `json:"Attachable"`
		Ingress    bool              `json:"Ingress"`
		ConfigOnly bool              `json:"ConfigOnly"`
		ConfigFrom map[string]string `json:"ConfigFrom"`
		EnableIPv4 bool              `json:"EnableIPv4"`
		EnableIPv6 bool              `json:"EnableIPv6"`
		IPAM       struct {
			Driver  string                   `json:"Driver"`
			Options map[string]string        `json:"Options"`
			Config  []map[string]interface{} `json:"Config"`
		} `json:"IPAM"`
		Options map[string]string `json:"Options"`
		Labels  map[string]string `json:"Labels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
		return err
	}

	// Set values back to Terraform state
	d.Set("name", network.Name)
	d.Set("driver", network.Driver)
	d.Set("scope", network.Scope)
	d.Set("internal", network.Internal)
	d.Set("attachable", network.Attachable)
	d.Set("ingress", network.Ingress)
	d.Set("config_only", network.ConfigOnly)
	d.Set("enable_ipv4", network.EnableIPv4)
	d.Set("enable_ipv6", network.EnableIPv6)

	if v, ok := network.ConfigFrom["Network"]; ok {
		d.Set("config_from", v)
	}

	// IPAM fields
	d.Set("ipam_driver", network.IPAM.Driver)
	d.Set("ipam_options", network.IPAM.Options)

	var ipamConfigList []map[string]interface{}
	for _, c := range network.IPAM.Config {
		entry := map[string]interface{}{}
		if subnet, ok := c["Subnet"]; ok {
			entry["subnet"] = subnet
		}
		if ipRange, ok := c["IPRange"]; ok {
			entry["ip_range"] = ipRange
		}
		if gateway, ok := c["Gateway"]; ok {
			entry["gateway"] = gateway
		}
		if aux, ok := c["AuxiliaryAddresses"]; ok {
			entry["auxiliary_addresses"] = aux
		}
		ipamConfigList = append(ipamConfigList, entry)
	}
	d.Set("ipam_config", ipamConfigList)

	// Options & Labels
	d.Set("options", network.Options)
	d.Set("labels", network.Labels)

	return nil
}

func resourceDockerNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker network: %s", string(body))
	}

	d.SetId("")
	return nil
}
