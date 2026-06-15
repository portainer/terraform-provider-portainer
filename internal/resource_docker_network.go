package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerNetworkCreate,
		ReadContext:   resourceDockerNetworkRead,
		DeleteContext: resourceDockerNetworkDelete,
		UpdateContext: nil,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				// Expect ID in format "<endpoint_id>:<network_id>"
				parts := strings.SplitN(d.Id(), ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("unexpected format of ID (%q), expected <endpoint_id>:<network_id>", d.Id())
				}
				endpointID, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid endpoint ID: %w", err)
				}
				_ = d.Set("endpoint_id", endpointID)
				d.SetId(parts[1])
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true, Description: "ID of the Portainer environment (Docker host or Swarm) where the network is created."},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Name of the Docker network."},
			"driver":      {Type: schema.TypeString, Optional: true, Default: "bridge", ForceNew: true, Description: "Driver used by the Docker network (e.g., bridge, overlay, macvlan, host, null)."},
			"scope":       {Type: schema.TypeString, Optional: true, Default: "local", ForceNew: true, Description: "Scope of the Docker network (local, global, or swarm)."},
			"internal":    {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether the network is restricted to internal use only (no external connectivity)."},
			"attachable":  {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether standalone containers may attach to this Swarm overlay network."},
			"ingress":     {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether the network is a Swarm ingress network used for the routing mesh."},
			"config_only": {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether this is a config-only network that contains only configuration for other networks."},
			"config_from": {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "Name of a config-only network from which to copy configuration."},
			"enable_ipv4": {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether IPv4 is enabled on the Docker network."},
			"enable_ipv6": {Type: schema.TypeBool, Optional: true, ForceNew: true, Description: "Whether IPv6 is enabled on the Docker network."},
			"options":     {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, Description: "Driver-specific options passed to the Docker network driver."},
			"labels":      {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true, Description: "Key/value labels attached to the Docker network."},
			"swarm_node_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Swarm node ID where the network operation is targeted, when applicable.",
			},
			"ipam_driver": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "default",
				ForceNew:    true,
				Description: "IPAM driver used to allocate IP addresses for the Docker network.",
			},
			"ipam_options": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				ForceNew:    true,
				Description: "Driver-specific options passed to the IPAM driver.",
			},
			"ipam_config": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Description: "List of IPAM configuration blocks defining subnets, gateways, and IP ranges.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnet": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Subnet in CIDR notation for the IPAM pool.",
						},
						"ip_range": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Sub-range of IP addresses within the subnet from which to allocate container IPs.",
						},
						"gateway": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Gateway IPv4 or IPv6 address for the subnet.",
						},
						"auxiliary_addresses": {
							Type:        schema.TypeMap,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Auxiliary IPv4 or IPv6 addresses used by the network driver, keyed by name.",
						},
					},
				},
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the Portainer resource control associated with this Docker network.",
			},
		},
	}
}

type dockerNetworkCreateResponse struct {
	ID        string `json:"Id"`
	Warning   string `json:"Warning"`
	Portainer struct {
		ResourceControl struct {
			Id int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func resourceDockerNetworkCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	var response dockerNetworkCreateResponse

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, headers, payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create docker network: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create docker network: %s", string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker network create response: %w", err))
	}

	d.SetId(response.ID)

	if response.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", response.Portainer.ResourceControl.Id)
	}

	return nil
}

// dockerNetworkSummary is a minimal representation used for fallback lookup.
type dockerNetworkSummary struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Scope  string `json:"Scope"`
}

// findDockerNetworkFallback tries to locate a network by name/driver/scope
// via /networks?filters=... when a lookup by ID failed (e.g. transient 404).
func findDockerNetworkFallback(d *schema.ResourceData, client *APIClient, endpointID int, headers map[string]string) (*dockerNetworkSummary, error) {
	name := d.Get("name").(string)
	driver := d.Get("driver").(string)
	scope := d.Get("scope").(string)

	filters := map[string]map[string]bool{
		"name":   {name: true},
		"driver": {driver: true},
		"scope":  {scope: true},
	}

	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker network filters: %w", err)
	}

	path := fmt.Sprintf(
		"/endpoints/%d/docker/networks?filters=%s",
		endpointID,
		url.QueryEscape(string(filtersJSON)),
	)

	resp, err := client.DoRequest(http.MethodGet, path, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list docker networks for fallback lookup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list docker networks for fallback lookup: %s", string(body))
	}

	var networks []dockerNetworkSummary
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return nil, fmt.Errorf("failed to decode docker networks list: %w", err)
	}

	if len(networks) == 1 {
		return &networks[0], nil
	}

	return nil, nil
}

func resourceDockerNetworkRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	networkID := d.Id()

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, networkID)
	resp, err := client.DoRequest(http.MethodGet, path, headers, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read docker network: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		network, err := findDockerNetworkFallback(d, client, endpointID, headers)
		if err != nil {
			return diag.FromErr(err)
		}
		if network == nil {
			d.SetId("")
			return nil
		}
		d.SetId(network.ID)
		return resourceDockerNetworkRead(ctx, d, meta)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read docker network: %s", string(body)))
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
		Portainer struct {
			ResourceControl struct {
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker network response: %w", err))
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

	if result.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", result.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerNetworkDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, headers, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete docker network: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete docker network: %s", string(body)))
	}

	d.SetId("")
	return nil
}
