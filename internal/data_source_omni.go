package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// omniMachineStatus is the machine shape Portainer returns from both the list
// and the inspect endpoint. The list data source surfaces the summary fields;
// the inspect one also unpacks the hardware and network detail.
type omniMachineStatus struct {
	MachineName string            `json:"machineName"`
	Labels      map[string]string `json:"labels"`
	Spec        struct {
		Cluster           string `json:"cluster"`
		Connected         bool   `json:"connected"`
		Maintenance       bool   `json:"maintenance"`
		ManagementAddress string `json:"management_address"`
		PowerState        string `json:"power_state"`
		Role              string `json:"role"`
		TalosVersion      string `json:"talos_version"`
		LastError         string `json:"last_error"`
		Hardware          struct {
			Arch         string `json:"arch"`
			BlockDevices []struct {
				LinuxName  string `json:"linux_name"`
				Model      string `json:"model"`
				Size       int64  `json:"size"`
				Type       string `json:"type"`
				SystemDisk bool   `json:"system_disk"`
			} `json:"blockdevices"`
			MemoryModules []struct {
				Description string `json:"description"`
				SizeMB      int    `json:"size_mb"`
			} `json:"memory_modules"`
			Processors []struct {
				Description  string `json:"description"`
				Manufacturer string `json:"manufacturer"`
				CoreCount    int    `json:"core_count"`
				ThreadCount  int    `json:"thread_count"`
				Frequency    int    `json:"frequency"`
			} `json:"processors"`
		} `json:"hardware"`
		Network struct {
			Hostname        string   `json:"hostname"`
			DomainName      string   `json:"domainname"`
			Addresses       []string `json:"addresses"`
			DefaultGateways []string `json:"default_gateways"`
			NetworkLinks    []struct {
				LinuxName       string `json:"linux_name"`
				HardwareAddress string `json:"hardware_address"`
				LinkUp          bool   `json:"link_up"`
				SpeedMbps       int    `json:"speed_mbps"`
			} `json:"network_links"`
		} `json:"network"`
		PlatformMetadata struct {
			Platform     string `json:"platform"`
			Hostname     string `json:"hostname"`
			InstanceID   string `json:"instance_id"`
			InstanceType string `json:"instance_type"`
			Region       string `json:"region"`
			Zone         string `json:"zone"`
		} `json:"platform_metadata"`
	} `json:"spec"`
}

// omniMachineSummary is the subset both data sources agree on. Listing is for
// finding a machine; the inspect data source is where the detail lives.
func omniMachineSummary(machine omniMachineStatus) map[string]interface{} {
	labels := make(map[string]interface{}, len(machine.Labels))
	for k, v := range machine.Labels {
		labels[k] = v
	}
	return map[string]interface{}{
		"machine_name":       machine.MachineName,
		"cluster":            machine.Spec.Cluster,
		"connected":          machine.Spec.Connected,
		"maintenance":        machine.Spec.Maintenance,
		"management_address": machine.Spec.ManagementAddress,
		"power_state":        machine.Spec.PowerState,
		"role":               machine.Spec.Role,
		"talos_version":      machine.Spec.TalosVersion,
		"last_error":         machine.Spec.LastError,
		"labels":             labels,
	}
}

func dataSourceOmniMachines() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniMachinesRead,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			// Computed attributes
			"machines": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The machines Omni knows about. Use `portainer_omni_machine` for the hardware and network detail of one of them.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"machine_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the machine.",
						},
						"cluster": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Cluster the machine belongs to, empty while it is unallocated.",
						},
						"connected": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the machine is currently connected to Omni.",
						},
						"maintenance": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the machine is in maintenance mode.",
						},
						"management_address": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Address Omni manages the machine on.",
						},
						"power_state": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Power state Omni reports for the machine.",
						},
						"role": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Role the machine plays in its cluster.",
						},
						"talos_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Talos version running on the machine.",
						},
						"last_error": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Most recent error Omni recorded for the machine, empty when there is none.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels Omni has on the machine.",
						},
					},
				},
			},
		},
	}
}

func dataSourceOmniMachinesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)

	var machines []omniMachineStatus
	listURL := fmt.Sprintf("%s/omni/%d/machines", client.Endpoint, credentialID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &machines); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the Omni machines: %w", err))
	}

	summaries := make([]interface{}, 0, len(machines))
	for _, machine := range machines {
		summaries = append(summaries, omniMachineSummary(machine))
	}

	if err := setFields(d, map[string]interface{}{"machines": summaries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-omni-machines-%d", credentialID))
	return nil
}

func dataSourceOmniMachine() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniMachineRead,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			"machine_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the machine to inspect.",
			},
			// Computed attributes
			"cluster": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Cluster the machine belongs to, empty while it is unallocated.",
			},
			"connected": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the machine is currently connected to Omni.",
			},
			"maintenance": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the machine is in maintenance mode.",
			},
			"management_address": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Address Omni manages the machine on.",
			},
			"power_state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Power state Omni reports for the machine.",
			},
			"role": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Role the machine plays in its cluster.",
			},
			"talos_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Talos version running on the machine.",
			},
			"last_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Most recent error Omni recorded for the machine, empty when there is none.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels Omni has on the machine.",
			},
			"architecture": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "CPU architecture of the machine.",
			},
			"hostname": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Hostname the machine reports.",
			},
			"domain_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Domain name the machine reports.",
			},
			"addresses": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Addresses configured on the machine.",
			},
			"default_gateways": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Default gateways configured on the machine.",
			},
			"platform": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Platform the machine runs on, as reported by its metadata.",
			},
			"instance_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Instance identifier from the platform metadata.",
			},
			"instance_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Instance type from the platform metadata.",
			},
			"region": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Region from the platform metadata.",
			},
			"zone": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Zone from the platform metadata.",
			},
			"block_devices": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Block devices Omni sees on the machine, which is where an install disk name comes from.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"linux_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Device name under Linux, for example `/dev/sda`.",
						},
						"model": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Model the device reports.",
						},
						"size": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Size of the device in bytes.",
						},
						"type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Type of the device.",
						},
						"system_disk": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether Talos is installed on this device.",
						},
					},
				},
			},
			"memory_modules": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Memory modules Omni sees on the machine.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Description the module reports.",
						},
						"size_mb": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Size of the module in megabytes.",
						},
					},
				},
			},
			"processors": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Processors Omni sees on the machine.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Description the processor reports.",
						},
						"manufacturer": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Manufacturer of the processor.",
						},
						"core_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Physical cores.",
						},
						"thread_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Hardware threads.",
						},
						"frequency": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Clock frequency the processor reports.",
						},
					},
				},
			},
			"network_links": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Network links Omni sees on the machine, which is where an interface name comes from.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"linux_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Interface name under Linux, for example `eth0`.",
						},
						"hardware_address": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "MAC address of the link.",
						},
						"link_up": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the link is up.",
						},
						"speed_mbps": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Negotiated link speed in megabits per second.",
						},
					},
				},
			},
		},
	}
}

func dataSourceOmniMachineRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)
	name := d.Get("machine_name").(string)

	var machine omniMachineStatus
	machineURL := fmt.Sprintf("%s/omni/%d/machine/%s", client.Endpoint, credentialID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, machineURL, nil, &machine); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read Omni machine %s: %w", name, err))
	}

	fields := omniMachineSummary(machine)
	fields["architecture"] = machine.Spec.Hardware.Arch
	fields["hostname"] = machine.Spec.Network.Hostname
	fields["domain_name"] = machine.Spec.Network.DomainName
	fields["addresses"] = machine.Spec.Network.Addresses
	fields["default_gateways"] = machine.Spec.Network.DefaultGateways
	fields["platform"] = machine.Spec.PlatformMetadata.Platform
	fields["instance_id"] = machine.Spec.PlatformMetadata.InstanceID
	fields["instance_type"] = machine.Spec.PlatformMetadata.InstanceType
	fields["region"] = machine.Spec.PlatformMetadata.Region
	fields["zone"] = machine.Spec.PlatformMetadata.Zone

	devices := make([]interface{}, 0, len(machine.Spec.Hardware.BlockDevices))
	for _, device := range machine.Spec.Hardware.BlockDevices {
		devices = append(devices, map[string]interface{}{
			"linux_name": device.LinuxName, "model": device.Model,
			"size": int(device.Size), "type": device.Type, "system_disk": device.SystemDisk,
		})
	}
	fields["block_devices"] = devices

	modules := make([]interface{}, 0, len(machine.Spec.Hardware.MemoryModules))
	for _, module := range machine.Spec.Hardware.MemoryModules {
		modules = append(modules, map[string]interface{}{
			"description": module.Description, "size_mb": module.SizeMB,
		})
	}
	fields["memory_modules"] = modules

	processors := make([]interface{}, 0, len(machine.Spec.Hardware.Processors))
	for _, processor := range machine.Spec.Hardware.Processors {
		processors = append(processors, map[string]interface{}{
			"description": processor.Description, "manufacturer": processor.Manufacturer,
			"core_count": processor.CoreCount, "thread_count": processor.ThreadCount,
			"frequency": processor.Frequency,
		})
	}
	fields["processors"] = processors

	links := make([]interface{}, 0, len(machine.Spec.Network.NetworkLinks))
	for _, link := range machine.Spec.Network.NetworkLinks {
		links = append(links, map[string]interface{}{
			"linux_name": link.LinuxName, "hardware_address": link.HardwareAddress,
			"link_up": link.LinkUp, "speed_mbps": link.SpeedMbps,
		})
	}
	fields["network_links"] = links

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-omni-machine-%d-%s", credentialID, name))
	return nil
}

func dataSourceOmniMachineLogs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniMachineLogsRead,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			"machine_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the machine to fetch logs for.",
			},
			// Computed attributes
			"logs": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The machine's logs as Omni returns them. They change on every read, so a configuration that depends on this value will show a difference on every plan.",
			},
		},
	}
}

func dataSourceOmniMachineLogsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)
	name := d.Get("machine_name").(string)

	// The endpoint answers with the log text itself rather than a JSON
	// document, so the raw body is what gets read.
	logsURL := fmt.Sprintf("%s/omni/%d/machine/%s/logs", client.Endpoint, credentialID, url.PathEscape(name))
	body, err := apiGETRaw(ctx, client, logsURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the logs of Omni machine %s: %w", name, err))
	}

	if err := setFields(d, map[string]interface{}{"logs": string(body)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-omni-machine-logs-%d-%s", credentialID, name))
	return nil
}

func dataSourceOmniTalosVersions() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniTalosVersionsRead,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			// Computed attributes
			"talos_versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The Talos versions the credential can provision, sorted.",
			},
			"compatibility": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "For each Talos version, the versions Portainer reports as compatible with it.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"talos_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "A Talos version on offer.",
						},
						"kubernetes_versions": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Kubernetes versions Portainer lists against that Talos version.",
						},
					},
				},
			},
		},
	}
}

func dataSourceOmniTalosVersionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)

	// Portainer answers with a map of version to the versions it lists against
	// it, so the keys are sorted to keep the data source's output stable.
	var versions map[string][]string
	versionsURL := fmt.Sprintf("%s/omni/%d/cluster/version/talos", client.Endpoint, credentialID)
	if err := doJSON(ctx, client, http.MethodGet, versionsURL, nil, &versions); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the supported Talos versions: %w", err))
	}

	talosVersions := make([]string, 0, len(versions))
	for version := range versions {
		talosVersions = append(talosVersions, version)
	}
	sort.Strings(talosVersions)

	compatibility := make([]interface{}, 0, len(talosVersions))
	list := make([]interface{}, 0, len(talosVersions))
	for _, version := range talosVersions {
		list = append(list, version)
		compatibility = append(compatibility, map[string]interface{}{
			"talos_version":       version,
			"kubernetes_versions": versions[version],
		})
	}

	if err := setFields(d, map[string]interface{}{
		"talos_versions": list,
		"compatibility":  compatibility,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-omni-talos-versions-%d", credentialID))
	return nil
}

func dataSourceOmniUpgradeStatus() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniUpgradeStatusRead,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the cluster to report on.",
			},
			"component": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"kubernetes", "talos"}, false),
				Description:  "Which upgrade to report on: `kubernetes` or `talos`. Portainer tracks the two separately and they can be at different steps.",
			},
			// Computed attributes
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Status Omni reports for the upgrade.",
			},
			"step": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Step the upgrade is currently on.",
			},
			"phase": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Phase Omni reports for the upgrade.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why the upgrade failed, empty when it has not.",
			},
			"current_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version the upgrade is moving to.",
			},
			"last_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version the cluster was on before the upgrade.",
			},
			"available_versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Versions the cluster can be upgraded to.",
			},
		},
	}
}

func dataSourceOmniUpgradeStatusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)
	cluster := d.Get("cluster").(string)
	component := d.Get("component").(string)

	// Portainer spells the Kubernetes half of this pair "k8s" in the path.
	suffix := "talos"
	if component == "kubernetes" {
		suffix = "k8s"
	}

	var status struct {
		Status                string   `json:"status"`
		Step                  string   `json:"step"`
		Phase                 int      `json:"phase"`
		Error                 string   `json:"error"`
		CurrentUpgradeVersion string   `json:"current_upgrade_version"`
		LastUpgradeVersion    string   `json:"last_upgrade_version"`
		UpgradeVersions       []string `json:"upgrade_versions"`
	}
	statusURL := fmt.Sprintf("%s/omni/%d/cluster/%s/upgrade/status/%s",
		client.Endpoint, credentialID, url.PathEscape(cluster), suffix)
	if err := doJSON(ctx, client, http.MethodGet, statusURL, nil, &status); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the %s upgrade status of Omni cluster %s: %w", component, cluster, err))
	}

	if err := setFields(d, map[string]interface{}{
		"status":             status.Status,
		"step":               status.Step,
		"phase":              status.Phase,
		"error":              status.Error,
		"current_version":    status.CurrentUpgradeVersion,
		"last_version":       status.LastUpgradeVersion,
		"available_versions": status.UpgradeVersions,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-omni-upgrade-status-%d-%s-%s", credentialID, cluster, component))
	return nil
}

func dataSourceOmniServiceAccount() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceOmniServiceAccountRead,

		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Omni instance to check, the value of `OMNI_ENDPOINT`.",
			},
			"service_account_key": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Omni service account key to check, the value of `OMNI_SERVICE_ACCOUNT_KEY`.",
			},
			"fail_on_error": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				// Defaults to true so a bad credential stops the plan rather
				// than letting a cluster resource fail half way through
				// provisioning.
				Description: "Whether a credential Omni rejects fails the plan. Defaults to `true`; set it to `false` to read `valid` and decide in the configuration.",
			},
			// Computed attributes
			"valid": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Omni accepted the service account.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why Omni rejected the service account, empty when it accepted it.",
			},
		},
	}
}

func dataSourceOmniServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	query := url.Values{}
	query.Set("endpoint", d.Get("endpoint").(string))
	query.Set("serviceAccountKey", d.Get("service_account_key").(string))
	validateURL := client.Endpoint + "/omni/serviceaccount/validate?" + query.Encode()

	valid := true
	message := ""
	if err := doJSON(ctx, client, http.MethodGet, validateURL, nil, nil); err != nil {
		valid = false
		message = err.Error()
		if d.Get("fail_on_error").(bool) {
			return diag.FromErr(fmt.Errorf("the Omni service account was rejected (set fail_on_error = false to read the result instead): %w", err))
		}
	}

	if err := setFields(d, map[string]interface{}{"valid": valid, "error": message}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-omni-service-account")
	return nil
}
