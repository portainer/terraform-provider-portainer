package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_docker_network creates a Docker network on an environment. Every
// attribute forces a new network.

var newDockerNetworkResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_network",
	schema:     dockerNetworkSchema,
	create:     dockerNetworkCreate,
	read:       dockerNetworkRead,
	delete:     dockerNetworkDelete,
	importer:   dockerNetworkImport,
	identity:   dockerNetworkIdentity,
})

func dockerNetworkSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"attachable":          schema.BoolAttribute{Optional: true, Description: "Whether standalone containers may attach to this Swarm overlay network.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"config_from":         schema.StringAttribute{Optional: true, Description: "Name of a config-only network from which to copy configuration.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"config_only":         schema.BoolAttribute{Optional: true, Description: "Whether this is a config-only network that contains only configuration for other networks.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"driver":              schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("bridge"), Description: "Driver used by the Docker network (e.g., bridge, overlay, macvlan, host, null).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"enable_ipv4":         schema.BoolAttribute{Optional: true, Description: "Whether IPv4 is enabled on the Docker network.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"enable_ipv6":         schema.BoolAttribute{Optional: true, Description: "Whether IPv6 is enabled on the Docker network.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"endpoint_id":         schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker host or Swarm) where the network is created.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"ingress":             schema.BoolAttribute{Optional: true, Description: "Whether the network is a Swarm ingress network used for the routing mesh.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"internal":            schema.BoolAttribute{Optional: true, Description: "Whether the network is restricted to internal use only (no external connectivity).", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"ipam_driver":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("default"), Description: "IPAM driver used to allocate IP addresses for the Docker network.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"ipam_options":        schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Driver-specific options passed to the IPAM driver.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"labels":              schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Key/value labels attached to the Docker network.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"name":                schema.StringAttribute{Required: true, Description: "Name of the Docker network.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"options":             schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Driver-specific options passed to the Docker network driver.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"resource_control_id": schema.Int64Attribute{Computed: true, Description: "ID of the Portainer resource control associated with this Docker network."},
			"scope":               schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("local"), Description: "Scope of the Docker network (local, global, or swarm).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"swarm_node_id":       schema.StringAttribute{Optional: true, Description: "Swarm node ID where the network operation is targeted, when applicable.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
		},
		Blocks: map[string]schema.Block{
			"ipam_config": schema.ListNestedBlock{
				Description:   "List of IPAM configuration blocks defining subnets, gateways, and IP ranges.",
				PlanModifiers: []planmodifier.List{dkReplaceList()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"auxiliary_addresses": schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Auxiliary IPv4 or IPv6 addresses used by the network driver, keyed by name."},
						"gateway":             schema.StringAttribute{Optional: true, Description: "Gateway IPv4 or IPv6 address for the subnet."},
						"ip_range":            schema.StringAttribute{Optional: true, Description: "Sub-range of IP addresses within the subnet from which to allocate container IPs."},
						"subnet":              schema.StringAttribute{Required: true, Description: "Subnet in CIDR notation for the IPAM pool."},
					},
				},
			},
		},
	}
}

type dockerNetworkCreateResponse struct {
	ID        string `json:"Id"`
	Warning   string `json:"Warning"`
	Portainer struct {
		ResourceControl struct {
			ID int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func dockerNetworkCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")

	payload := map[string]interface{}{
		"Name":       dkString(d, "name"),
		"Driver":     dkString(d, "driver"),
		"ConfigOnly": dkBool(d, "config_only"),
	}

	configOnly := dkBool(d, "config_only")

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
		"Driver": dkString(d, "ipam_driver"),
	}
	if v, ok := d.GetOk("ipam_options"); ok {
		ipam["Options"] = v.(map[string]interface{})
	}
	if v, ok := d.GetOk("ipam_config"); ok {
		configList := v.([]interface{})
		ipamConfigs := make([]map[string]interface{}, 0, len(configList))
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
	resp, err := client.DoRequest(ctx, http.MethodPost, path, headers, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker network: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode docker network create response: %w", err)
	}

	d.SetID(response.ID)

	if response.Portainer.ResourceControl.ID != 0 {
		fields := map[string]interface{}{
			"resource_control_id": response.Portainer.ResourceControl.ID,
		}
		if err := d.SetFields(fields); err != nil {
			return err
		}
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
func findDockerNetworkFallback(ctx context.Context, d *dkBag, client *APIClient, endpointID int, headers map[string]string) (*dockerNetworkSummary, error) {
	name := dkString(d, "name")
	driver := dkString(d, "driver")
	scope := dkString(d, "scope")

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

	resp, err := client.DoRequest(ctx, http.MethodGet, path, headers, nil)
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

func dockerNetworkRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	networkID := d.ID()

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, networkID)
	resp, err := client.DoRequest(ctx, http.MethodGet, path, headers, nil)
	if err != nil {
		return fmt.Errorf("failed to read docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		network, err := findDockerNetworkFallback(ctx, d, client, endpointID, headers)
		if err != nil {
			return err
		}
		if network == nil {
			d.SetID("")
			return nil
		}
		d.SetID(network.ID)
		return dockerNetworkRead(ctx, d, client)
	}

	if resp.StatusCode != http.StatusOK {
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
		Portainer struct {
			ResourceControl struct {
				ID int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode docker network response: %w", err)
	}

	configOnly := result.ConfigOnly

	driver := result.Driver
	scope := result.Scope
	if configOnly {
		driver = dkString(d, "driver")
		scope = dkString(d, "scope")
	}
	fields := map[string]interface{}{
		"driver": driver,
		"scope":  scope,
		// name a config_only
		"name":        result.Name,
		"config_only": configOnly,
	}

	if !configOnly {
		fields["internal"] = result.Internal
		fields["attachable"] = result.Attachable
		fields["ingress"] = result.Ingress
		enableIPv4 := result.EnableIPv4
		enableIPv6 := result.EnableIPv6
		if !enableIPv4 && !enableIPv6 {
			if dkBool(d, "enable_ipv4") {
				enableIPv4 = true
			}
			if dkBool(d, "enable_ipv6") {
				enableIPv6 = true
			}
		}
		// Docker 28+ enables IPv4 on every network unless told otherwise, so
		// reading the flags into an attribute the configuration leaves out
		// would plan a replacement on every run: they are refreshed only once
		// the state tracks them.
		if !dkStateNull(d, "enable_ipv4") {
			fields["enable_ipv4"] = enableIPv4
		}
		if !dkStateNull(d, "enable_ipv6") {
			fields["enable_ipv6"] = enableIPv6
		}
	} else {
		fields["internal"] = dkBool(d, "internal")
		fields["attachable"] = dkBool(d, "attachable")
		fields["ingress"] = dkBool(d, "ingress")
		fields["enable_ipv4"] = dkBool(d, "enable_ipv4")
		fields["enable_ipv6"] = dkBool(d, "enable_ipv6")
	}

	// options
	if len(result.Options) == 0 {
		if v, ok := d.GetOk("options"); ok {
			fields["options"] = v
		}
	} else {
		fields["options"] = dockerNetworkOptions(result.Options, dkMap(d, "options"))
	}

	// labels
	if len(result.Labels) == 0 {
		if v, ok := d.GetOk("labels"); ok {
			fields["labels"] = v
		}
	} else {
		labels := make(map[string]interface{}, len(result.Labels))
		for k, v := range result.Labels {
			labels[k] = v
		}
		fields["labels"] = labels
	}

	// IPAM
	fields["ipam_driver"] = result.IPAM.Driver

	if len(result.IPAM.Options) == 0 {
		if v, ok := d.GetOk("ipam_options"); ok {
			fields["ipam_options"] = v
		}
	} else {
		ipamOpts := make(map[string]interface{}, len(result.IPAM.Options))
		for k, v := range result.IPAM.Options {
			ipamOpts[k] = v
		}
		fields["ipam_options"] = ipamOpts
	}

	if result.Portainer.ResourceControl.ID != 0 {
		fields["resource_control_id"] = result.Portainer.ResourceControl.ID
	}

	if err := d.SetFields(fields); err != nil {
		return err
	}

	// ipam_config: the Docker API returns Docker-style keys (Subnet/Gateway/IPRange)
	// that don't match this resource's schema keys (subnet/gateway/ip_range), so this
	// Set has always failed and ipam_config is not refreshed from the API. Keep it
	// best-effort (log, don't fail) to preserve prior behavior — fixing the mapping
	// would change state for existing users and belongs in a separate change.
	if err := d.Set("ipam_config", result.IPAM.Config); err != nil {
		log.Printf("[WARN] docker_network %s: skipping ipam_config refresh (schema mismatch): %v", d.ID(), err)
	}

	return nil
}

// dockerNetworkDerivedOptions are the options Docker (28+) adds to every
// network from EnableIPv4/EnableIPv6, which enable_ipv4/enable_ipv6 already
// track.
var dockerNetworkDerivedOptions = []string{
	"com.docker.network.enable_ipv4",
	"com.docker.network.enable_ipv6",
}

// dockerNetworkOptions drops the options Docker derives itself, unless they
// were configured, so a network nobody touched does not plan a replacement.
func dockerNetworkOptions(actual, configured map[string]interface{}) map[string]interface{} {
	options := make(map[string]interface{}, len(actual))
	for k, v := range actual {
		if _, ok := configured[k]; !ok && slices.Contains(dockerNetworkDerivedOptions, k) {
			continue
		}
		options[k] = v
	}
	return options
}

func dockerNetworkDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	headers := map[string]string{}
	if nodeID, ok := d.GetOk("swarm_node_id"); ok && nodeID.(string) != "" {
		headers["X-PortainerAgent-Target"] = nodeID.(string)
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, id)
	resp, err := client.DoRequest(ctx, http.MethodDelete, path, headers, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker network: %s", string(body))
	}

	d.SetID("")
	return nil
}

// dockerNetworkImport takes "<endpoint_id>:<network_id>".
func dockerNetworkImport(_ context.Context, d *dkBag, _ *APIClient) error {
	parts := strings.SplitN(d.ID(), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("unexpected format of ID (%q), expected <endpoint_id>:<network_id>", d.ID())
	}
	endpointID, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid endpoint ID: %w", err)
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	d.SetID(parts[1])
	return nil
}
