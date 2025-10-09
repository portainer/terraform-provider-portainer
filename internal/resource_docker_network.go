package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerNetworkCreate,
		Read:   resourceDockerNetworkRead,
		Delete: resourceDockerNetworkDelete,
		Update: nil,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				// Expect ID in format "<endpoint_id>:<network_id>"
				parts := strings.SplitN(d.Id(), ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("unexpected format of ID (%q), expected <endpoint_id>:<network_id>", d.Id())
				}
				endpointID, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid endpoint ID: %w", err)
				}
				d.Set("endpoint_id", endpointID)
				d.SetId(parts[1])
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true},
			"driver":      {Type: schema.TypeString, Optional: true, Default: "bridge", ForceNew: true},
			"scope":       {Type: schema.TypeString, Optional: true, Default: "local", ForceNew: true},
			"internal":    {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"attachable":  {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"ingress":     {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"config_only": {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"config_from": {Type: schema.TypeString, Optional: true, ForceNew: true},
			"enable_ipv4": {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"enable_ipv6": {Type: schema.TypeBool, Optional: true, ForceNew: true},
			"options":     {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"labels":      {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"swarm_node_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
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
		"ConfigOnly": d.Get("config_only").(bool),
	}

	configOnly := d.Get("config_only").(bool)

	if !configOnly {
		if v, ok := d.GetOk("internal"); ok {
			payload["Internal"] = v.(bool)
		}
		if v, ok := d.GetOk("attachable"); ok {
			payload["Attachable"] = v.(bool)
		}
		if v, ok := d.GetOk("ingress"); ok {
			payload["Ingress"] = v.(bool)
		}
		if v, ok := d.GetOk("enable_ipv4"); ok {
			payload["EnableIPv4"] = v.(bool)
		}
		if v, ok := d.GetOk("enable_ipv6"); ok {
			payload["EnableIPv6"] = v.(bool)
		}
		if v, ok := d.GetOk("scope"); ok {
			payload["Scope"] = v.(string)
		}
	}

	if v, ok := d.GetOk("options"); ok {
		payload["Options"] = v.(map[string]interface{})
	}
	if v, ok := d.GetOk("labels"); ok {
		payload["Labels"] = v.(map[string]interface{})
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

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, headers, payload)
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
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read docker network: %s", string(body))
	}

	var result struct {
		Name       string                 `json:"Name"`
		Driver     string                 `json:"Driver"`
		Scope      string                 `json:"Scope"`
		Internal   bool                   `json:"Internal"`
		Attachable bool                   `json:"Attachable"`
		Ingress    bool                   `json:"Ingress"`
		ConfigOnly bool                   `json:"ConfigOnly"`
		EnableIPv4 bool                   `json:"EnableIPv4"`
		EnableIPv6 bool                   `json:"EnableIPv6"`
		Options    map[string]interface{} `json:"Options"`
		Labels     map[string]string      `json:"Labels"`
		IPAM       struct {
			Driver  string                   `json:"Driver"`
			Options map[string]string        `json:"Options"`
			Config  []map[string]interface{} `json:"Config"`
		} `json:"IPAM"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode docker network response: %w", err)
	}

	configOnly := result.ConfigOnly

	driver := result.Driver
	scope := result.Scope
	if configOnly {
		driver = d.Get("driver").(string)
		scope = d.Get("scope").(string)
	}
	_ = d.Set("driver", driver)
	_ = d.Set("scope", scope)

	// name a config_only
	_ = d.Set("name", result.Name)
	_ = d.Set("config_only", configOnly)

	if !configOnly {
		_ = d.Set("internal", result.Internal)
		_ = d.Set("attachable", result.Attachable)
		_ = d.Set("ingress", result.Ingress)
		enableIPv4 := result.EnableIPv4
		enableIPv6 := result.EnableIPv6
		if !enableIPv4 && !enableIPv6 {
			if d.Get("enable_ipv4").(bool) {
				enableIPv4 = true
			}
			if d.Get("enable_ipv6").(bool) {
				enableIPv6 = true
			}
		}
		_ = d.Set("enable_ipv4", enableIPv4)
		_ = d.Set("enable_ipv6", enableIPv6)
	} else {
		_ = d.Set("internal", d.Get("internal").(bool))
		_ = d.Set("attachable", d.Get("attachable").(bool))
		_ = d.Set("ingress", d.Get("ingress").(bool))
		_ = d.Set("enable_ipv4", d.Get("enable_ipv4").(bool))
		_ = d.Set("enable_ipv6", d.Get("enable_ipv6").(bool))
	}

	// options
	if len(result.Options) == 0 {
		if v, ok := d.GetOk("options"); ok {
			_ = d.Set("options", v)
		}
	} else {
		_ = d.Set("options", result.Options)
	}

	// labels
	if len(result.Labels) == 0 {
		if v, ok := d.GetOk("labels"); ok {
			_ = d.Set("labels", v)
		}
	} else {
		labels := make(map[string]interface{}, len(result.Labels))
		for k, v := range result.Labels {
			labels[k] = v
		}
		_ = d.Set("labels", labels)
	}

	// IPAM
	_ = d.Set("ipam_driver", result.IPAM.Driver)

	if len(result.IPAM.Options) == 0 {
		if v, ok := d.GetOk("ipam_options"); ok {
			_ = d.Set("ipam_options", v)
		}
	} else {
		ipamOpts := make(map[string]interface{}, len(result.IPAM.Options))
		for k, v := range result.IPAM.Options {
			ipamOpts[k] = v
		}
		_ = d.Set("ipam_options", ipamOpts)
	}

	_ = d.Set("ipam_config", result.IPAM.Config)

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
