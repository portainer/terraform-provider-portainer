package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceOmniCluster manages a Talos cluster provisioned through Siderolabs
// Omni. Creating one also creates the Portainer environment that fronts it,
// which is why endpoint_id is reported back.
//
// Portainer's read only returns the cluster's spec and status, never the
// machines it was built from, so the control plane and worker blocks are
// configuration-only: they are never read back, and drift in the machine list
// is not something this resource can detect.
func resourceOmniCluster() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceOmniClusterCreate,
		ReadContext:   resourceOmniClusterRead,
		UpdateContext: resourceOmniClusterUpdate,
		DeleteContext: resourceOmniClusterDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the cluster. Omni cannot rename a cluster, so changing it replaces the resource.",
			},
			"kind": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Omni resource kind of the cluster. Leave unset unless Omni asks for a specific one.",
			},
			"kubernetes_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Kubernetes version to run. Leave unset to let Omni pick, and read the result back from this attribute.",
			},
			"talos_version": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Talos version to run. Use the `portainer_omni_talos_versions` data source to see what the credential supports.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels to set on the Omni cluster resource.",
			},
			"cluster_config": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				// Omni accepts either a structured spec or a raw template. The
				// raw one is only read at creation, so changing it replaces.
				Description: "Raw Omni cluster template, as an alternative to describing the cluster with the blocks below. Only read when the cluster is created.",
			},
			"portainer_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "URL the provisioned nodes reach Portainer on. Leave unset to use the instance's configured URL.",
			},
			"tunnel_server_address": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Address the provisioned nodes reach Portainer's tunnel server on.",
			},
			"validate_before_create": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				// Provisioning a Talos cluster touches real machines and takes
				// a long time. Portainer offers a cheap validation call, so the
				// default is to use it rather than to find out half way through.
				Description: "Whether to validate the cluster configuration with Portainer before provisioning it. Defaults to `true`.",
			},
			"cluster_patch": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Talos configuration patches to apply to the cluster as a whole.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id_override": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Identifier Omni stores the patch under, instead of the generated one.",
						},
						"annotation_name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Name annotation Omni shows for the patch.",
						},
						"inline": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validateJSONObject,
							Description:  "Patch body as a JSON object. Use `jsonencode({...})`.",
						},
					},
				},
			},
			"control_plane": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "The cluster's control plane.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"kind": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Omni resource kind of the control plane. Leave unset unless Omni asks for a specific one.",
						},
						"machine": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Machines that make up the cluster's control plane.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Name of the machine as Omni knows it.",
									},
									"hostname": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Hostname to configure on the machine.",
									},
									"kind": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Omni resource kind of the machine. Leave unset unless Omni asks for a specific one.",
									},
									"install_disk": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Block device Talos is installed onto, for example `/dev/sda`.",
									},
									"nameservers": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "DNS servers to configure on the machine.",
									},
									"system_disk_size": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "Size of the ephemeral system volume in GiB. Omni turns this into a volume configuration patch.",
									},
									"user_disk": {
										Type:        schema.TypeList,
										Optional:    true,
										MaxItems:    1,
										Description: "A user volume to carve out of the remaining disk space.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"volume_name": {
													Type:        schema.TypeString,
													Required:    true,
													Description: "Name of the volume.",
												},
												"size": {
													Type:        schema.TypeInt,
													Optional:    true,
													Description: "Size of the volume in GiB. Zero means take all the space that is left.",
												},
											},
										},
									},
									"interface": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Network interfaces to configure on the machine.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"interface": {
													Type:        schema.TypeString,
													Required:    true,
													Description: "Name of the interface, for example `eth0`.",
												},
												"dhcp": {
													Type:        schema.TypeBool,
													Optional:    true,
													Description: "Whether the interface takes its address from DHCP. Leave `addresses` unset when it does.",
												},
												"addresses": {
													Type:        schema.TypeList,
													Optional:    true,
													Elem:        &schema.Schema{Type: schema.TypeString},
													Description: "Static addresses in CIDR notation, for example `10.0.0.10/24`.",
												},
												"route": {
													Type:        schema.TypeList,
													Optional:    true,
													Description: "Static routes to add through the interface.",
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"network": {
																Type:        schema.TypeString,
																Required:    true,
																Description: "Destination network in CIDR notation. Use `0.0.0.0/0` for a default route.",
															},
															"gateway": {
																Type:        schema.TypeString,
																Required:    true,
																Description: "Gateway the route goes through.",
															},
														},
													},
												},
											},
										},
									},
									"patch": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Talos machine configuration patches to apply.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"id_override": {
													Type:        schema.TypeString,
													Optional:    true,
													Description: "Identifier Omni stores the patch under, instead of the generated one.",
												},
												"inline": {
													Type:         schema.TypeString,
													Optional:     true,
													ValidateFunc: validateJSONObject,
													Description:  "Patch body as a JSON object. Use `jsonencode({...})`.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"worker": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "The cluster's worker pool.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Name of the worker pool.",
						},
						"kind": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Omni resource kind of the worker pool. Leave unset unless Omni asks for a specific one.",
						},
						"machine": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Machines that make up this worker pool.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Name of the machine as Omni knows it.",
									},
									"hostname": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Hostname to configure on the machine.",
									},
									"kind": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Omni resource kind of the machine. Leave unset unless Omni asks for a specific one.",
									},
									"install_disk": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Block device Talos is installed onto, for example `/dev/sda`.",
									},
									"nameservers": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "DNS servers to configure on the machine.",
									},
									"system_disk_size": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "Size of the ephemeral system volume in GiB. Omni turns this into a volume configuration patch.",
									},
									"user_disk": {
										Type:        schema.TypeList,
										Optional:    true,
										MaxItems:    1,
										Description: "A user volume to carve out of the remaining disk space.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"volume_name": {
													Type:        schema.TypeString,
													Required:    true,
													Description: "Name of the volume.",
												},
												"size": {
													Type:        schema.TypeInt,
													Optional:    true,
													Description: "Size of the volume in GiB. Zero means take all the space that is left.",
												},
											},
										},
									},
									"interface": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Network interfaces to configure on the machine.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"interface": {
													Type:        schema.TypeString,
													Required:    true,
													Description: "Name of the interface, for example `eth0`.",
												},
												"dhcp": {
													Type:        schema.TypeBool,
													Optional:    true,
													Description: "Whether the interface takes its address from DHCP. Leave `addresses` unset when it does.",
												},
												"addresses": {
													Type:        schema.TypeList,
													Optional:    true,
													Elem:        &schema.Schema{Type: schema.TypeString},
													Description: "Static addresses in CIDR notation, for example `10.0.0.10/24`.",
												},
												"route": {
													Type:        schema.TypeList,
													Optional:    true,
													Description: "Static routes to add through the interface.",
													Elem: &schema.Resource{
														Schema: map[string]*schema.Schema{
															"network": {
																Type:        schema.TypeString,
																Required:    true,
																Description: "Destination network in CIDR notation. Use `0.0.0.0/0` for a default route.",
															},
															"gateway": {
																Type:        schema.TypeString,
																Required:    true,
																Description: "Gateway the route goes through.",
															},
														},
													},
												},
											},
										},
									},
									"patch": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Talos machine configuration patches to apply.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"id_override": {
													Type:        schema.TypeString,
													Optional:    true,
													Description: "Identifier Omni stores the patch under, instead of the generated one.",
												},
												"inline": {
													Type:         schema.TypeString,
													Optional:     true,
													ValidateFunc: validateJSONObject,
													Description:  "Patch body as a JSON object. Use `jsonencode({...})`.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			// Computed attributes
			"endpoint_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer environment created for the cluster.",
			},
			"phase": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Provisioning phase Omni reports for the cluster.",
			},
			"ready": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Omni reports the cluster as ready.",
			},
			"available": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Omni reports the cluster as available.",
			},
			"control_plane_ready": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the control plane has come up.",
			},
			"kubernetes_api_ready": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the cluster's Kubernetes API is answering.",
			},
			"machines_total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Machines Omni counts in the cluster.",
			},
			"machines_requested": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Machines the cluster asked for.",
			},
			"machines_connected": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Machines currently connected to Omni.",
			},
			"machines_healthy": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Machines Omni considers healthy.",
			},
			"disk_encryption": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether disk encryption is enabled on the cluster.",
			},
			"workload_proxy_enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Omni's workload proxy is enabled for the cluster.",
			},
			"embedded_discovery_service": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the cluster uses Omni's embedded discovery service.",
			},
			"backup_enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Omni backs the cluster's etcd up on a schedule.",
			},
			"backup_interval": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Interval between the scheduled etcd backups.",
			},
		},
	}
}

// omniMachinesFromConfig turns a machine block list into the payload Omni
// expects. Every optional field is omitted when empty so a machine definition
// never sends a zero value Omni would read as a real setting.
func omniMachinesFromConfig(raw interface{}) ([]map[string]interface{}, error) {
	list, _ := raw.([]interface{})
	machines := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		machine := map[string]interface{}{"name": block["name"]}
		for field, key := range map[string]string{
			"hostname": "hostname",
			"kind":     "kind",
		} {
			if v, ok := block[field].(string); ok && v != "" {
				machine[key] = v
			}
		}
		if v, ok := block["install_disk"].(string); ok && v != "" {
			machine["install"] = map[string]interface{}{"disk": v}
		}
		if v, ok := block["nameservers"].([]interface{}); ok && len(v) > 0 {
			machine["nameservers"] = v
		}
		if v, ok := block["system_disk_size"].(int); ok && v != 0 {
			machine["systemDiskSize"] = v
		}

		if disks, ok := block["user_disk"].([]interface{}); ok && len(disks) > 0 {
			if disk, ok := disks[0].(map[string]interface{}); ok {
				machine["userDisk"] = map[string]interface{}{
					"volumeName": disk["volume_name"],
					// Zero is meaningful here: Omni reads it as "grow into
					// whatever space is left", so it is always sent.
					"size": disk["size"],
				}
			}
		}

		if interfaces, ok := block["interface"].([]interface{}); ok && len(interfaces) > 0 {
			encoded := make([]map[string]interface{}, 0, len(interfaces))
			for _, raw := range interfaces {
				iface, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				entry := map[string]interface{}{"interface": iface["interface"]}
				if v, ok := iface["dhcp"].(bool); ok && v {
					entry["dhcp"] = v
				}
				if v, ok := iface["addresses"].([]interface{}); ok && len(v) > 0 {
					entry["addresses"] = v
				}
				if routes, ok := iface["route"].([]interface{}); ok && len(routes) > 0 {
					encodedRoutes := make([]map[string]interface{}, 0, len(routes))
					for _, raw := range routes {
						route, ok := raw.(map[string]interface{})
						if !ok {
							continue
						}
						encodedRoutes = append(encodedRoutes, map[string]interface{}{
							"network": route["network"],
							"gateway": route["gateway"],
						})
					}
					entry["routes"] = encodedRoutes
				}
				encoded = append(encoded, entry)
			}
			machine["interfaces"] = encoded
		}

		patches, err := omniMachinePatches(block["patch"])
		if err != nil {
			return nil, fmt.Errorf("machine %v: %w", block["name"], err)
		}
		if len(patches) > 0 {
			machine["patches"] = patches
		}

		machines = append(machines, machine)
	}
	return machines, nil
}

// omniMachinePatches decodes the inline patch bodies, which are JSON in the
// configuration and objects on the wire.
func omniMachinePatches(raw interface{}) ([]map[string]interface{}, error) {
	list, _ := raw.([]interface{})
	patches := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		patch := map[string]interface{}{}
		if v, ok := block["id_override"].(string); ok && v != "" {
			patch["idOverride"] = v
		}
		if v, ok := block["inline"].(string); ok && v != "" {
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse the patch body as a JSON object: %w", err)
			}
			patch["inline"] = decoded
		}
		if len(patch) > 0 {
			patches = append(patches, patch)
		}
	}
	return patches, nil
}

// omniControlPlane and omniWorker build the two machine groups. They return
// false when the block is absent, which is how the payload leaves it out.
func omniControlPlane(raw interface{}) (map[string]interface{}, bool, error) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false, nil
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	machines, err := omniMachinesFromConfig(block["machine"])
	if err != nil {
		return nil, false, err
	}
	group := map[string]interface{}{"machines": machines}
	if v, ok := block["kind"].(string); ok && v != "" {
		group["kind"] = v
	}
	return group, true, nil
}

func omniWorker(raw interface{}) (map[string]interface{}, bool, error) {
	group, ok, err := omniControlPlane(raw)
	if !ok || err != nil {
		return nil, ok, err
	}
	list, _ := raw.([]interface{})
	if block, ok := list[0].(map[string]interface{}); ok {
		if v, ok := block["name"].(string); ok && v != "" {
			group["name"] = v
		}
	}
	return group, true, nil
}

// omniClusterSpec builds the cluster half of the payload, which create,
// update and validate all send.
func omniClusterSpec(d *schema.ResourceData) (map[string]interface{}, error) {
	cluster := map[string]interface{}{"name": d.Get("name").(string)}
	if v, ok := d.GetOk("kind"); ok && v.(string) != "" {
		cluster["kind"] = v.(string)
	}
	if v, ok := d.GetOk("kubernetes_version"); ok && v.(string) != "" {
		cluster["kubernetes"] = map[string]interface{}{"version": v.(string)}
	}
	if v, ok := d.GetOk("talos_version"); ok && v.(string) != "" {
		cluster["talos"] = map[string]interface{}{"version": v.(string)}
	}
	if v, ok := d.GetOk("labels"); ok {
		if labels, ok := v.(map[string]interface{}); ok && len(labels) > 0 {
			cluster["labels"] = labels
		}
	}

	list, _ := d.Get("cluster_patch").([]interface{})
	patches := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		patch := map[string]interface{}{}
		if v, ok := block["id_override"].(string); ok && v != "" {
			patch["idOverride"] = v
		}
		if v, ok := block["annotation_name"].(string); ok && v != "" {
			patch["annotations"] = map[string]interface{}{"name": v}
		}
		if v, ok := block["inline"].(string); ok && v != "" {
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse a cluster patch body as a JSON object: %w", err)
			}
			patch["inline"] = decoded
		}
		if len(patch) > 0 {
			patches = append(patches, patch)
		}
	}
	if len(patches) > 0 {
		cluster["patches"] = patches
	}

	return cluster, nil
}

// omniClusterID and omniClusterParts keep the resource ID - credential and
// cluster name - in one place, since every call needs both.
func omniClusterID(credentialID int, name string) string {
	return fmt.Sprintf("%d/%s", credentialID, name)
}

func omniClusterParts(id string) (int, string, error) {
	credential, name, found := strings.Cut(id, "/")
	if !found || credential == "" || name == "" {
		return 0, "", fmt.Errorf("the resource ID must be <credential id>/<cluster name>, got %q", id)
	}
	var credentialID int
	if _, err := fmt.Sscanf(credential, "%d", &credentialID); err != nil {
		return 0, "", fmt.Errorf("the credential part of the resource ID must be a number, got %q", credential)
	}
	return credentialID, name, nil
}

func resourceOmniClusterCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)

	cluster, err := omniClusterSpec(d)
	if err != nil {
		return diag.FromErr(err)
	}
	controlPlane, hasControlPlane, err := omniControlPlane(d.Get("control_plane"))
	if err != nil {
		return diag.FromErr(err)
	}
	worker, hasWorker, err := omniWorker(d.Get("worker"))
	if err != nil {
		return diag.FromErr(err)
	}

	payload := map[string]interface{}{"cluster": cluster}
	if hasControlPlane {
		payload["controlPlane"] = controlPlane
	}
	if hasWorker {
		payload["worker"] = worker
	}
	if v, ok := d.GetOk("cluster_config"); ok && v.(string) != "" {
		payload["clusterConfig"] = v.(string)
	}
	if v, ok := d.GetOk("portainer_url"); ok && v.(string) != "" {
		payload["portainerUrl"] = v.(string)
	}
	if v, ok := d.GetOk("tunnel_server_address"); ok && v.(string) != "" {
		payload["tunnelServerAddr"] = v.(string)
	}

	// Validation is a cheap call that catches a bad specification before any
	// machine is touched, so it runs first unless it was turned off.
	if d.Get("validate_before_create").(bool) {
		validateURL := fmt.Sprintf("%s/omni/%d/cluster/validate", client.Endpoint, credentialID)
		if err := doJSON(ctx, client, http.MethodPost, validateURL, payload, nil); err != nil {
			return diag.FromErr(fmt.Errorf("the Omni cluster configuration was rejected before provisioning started (set validate_before_create = false to skip this check): %w", err))
		}
	}

	var environment struct {
		ID int `json:"Id"`
	}
	createURL := fmt.Sprintf("%s/omni/%d/cluster/create", client.Endpoint, credentialID)
	if err := doJSON(ctx, client, http.MethodPost, createURL, payload, &environment); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create Omni cluster %s: %w", d.Get("name").(string), err))
	}

	if err := d.Set("endpoint_id", environment.ID); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(omniClusterID(credentialID, d.Get("name").(string)))
	return resourceOmniClusterRead(ctx, d, meta)
}

func resourceOmniClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	credentialID, name, err := omniClusterParts(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

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

	readURL := fmt.Sprintf("%s/omni/%d/cluster/%s", client.Endpoint, credentialID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, readURL, nil, &cluster); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read Omni cluster %s: %w", name, err))
	}

	// The machine blocks are deliberately absent from this map. Portainer's
	// read reports the cluster's spec and status but not the machines it was
	// built from, so writing anything into them would be a guess.
	if err := setFields(d, map[string]interface{}{
		"credential_id":              credentialID,
		"name":                       name,
		"kubernetes_version":         cluster.Metadata.KubernetesVersion,
		"talos_version":              cluster.Metadata.TalosVersion,
		"disk_encryption":            cluster.Metadata.Features.DiskEncryption,
		"workload_proxy_enabled":     cluster.Metadata.Features.EnableWorkloadProxy,
		"embedded_discovery_service": cluster.Metadata.Features.UseEmbeddedDiscoveryService,
		"backup_enabled":             cluster.Metadata.BackupConfiguration.Enabled,
		"backup_interval":            cluster.Metadata.BackupConfiguration.Interval,
		"available":                  cluster.Status.Available,
		"ready":                      cluster.Status.Ready,
		"phase":                      cluster.Status.Phase,
		"control_plane_ready":        cluster.Status.ControlPlaneReady,
		"kubernetes_api_ready":       cluster.Status.KubernetesAPIReady,
		"machines_total":             cluster.Status.Machines.Total,
		"machines_requested":         cluster.Status.Machines.Requested,
		"machines_connected":         cluster.Status.Machines.Connected,
		"machines_healthy":           cluster.Status.Machines.Healthy,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// omniMachineNames lists the machine names in a group block, which is what the
// update endpoint removes machines by.
func omniMachineNames(raw interface{}) []string {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil
	}
	machines, _ := block["machine"].([]interface{})
	names := make([]string, 0, len(machines))
	for _, item := range machines {
		if machine, ok := item.(map[string]interface{}); ok {
			if name, ok := machine["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// omniGroupDelta works out which machines to add and which to remove. The
// update endpoint is imperative - it takes machines to add and names to remove
// - so the declarative machine list has to be turned into that difference.
func omniGroupDelta(before, after interface{}) (added []interface{}, removed []string) {
	beforeNames := map[string]bool{}
	for _, name := range omniMachineNames(before) {
		beforeNames[name] = true
	}
	afterNames := map[string]bool{}
	for _, name := range omniMachineNames(after) {
		afterNames[name] = true
	}

	for _, name := range omniMachineNames(before) {
		if !afterNames[name] {
			removed = append(removed, name)
		}
	}

	if list, ok := after.([]interface{}); ok && len(list) > 0 {
		if block, ok := list[0].(map[string]interface{}); ok {
			machines, _ := block["machine"].([]interface{})
			for _, item := range machines {
				machine, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if name, ok := machine["name"].(string); ok && !beforeNames[name] {
					added = append(added, machine)
				}
			}
		}
	}
	return added, removed
}

// omniGroupFromMachines wraps a bare machine list back into the group shape
// the update payload takes.
func omniGroupFromMachines(source interface{}, machines []interface{}) (map[string]interface{}, error) {
	encoded, err := omniMachinesFromConfig(machines)
	if err != nil {
		return nil, err
	}
	group := map[string]interface{}{"machines": encoded}
	if list, ok := source.([]interface{}); ok && len(list) > 0 {
		if block, ok := list[0].(map[string]interface{}); ok {
			if v, ok := block["kind"].(string); ok && v != "" {
				group["kind"] = v
			}
			if v, ok := block["name"].(string); ok && v != "" {
				group["name"] = v
			}
		}
	}
	return group, nil
}

func resourceOmniClusterUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	credentialID, _, err := omniClusterParts(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	cluster, err := omniClusterSpec(d)
	if err != nil {
		return diag.FromErr(err)
	}
	payload := map[string]interface{}{"cluster": cluster}

	oldControlPlane, newControlPlane := d.GetChange("control_plane")
	addedControlPlanes, removedControlPlanes := omniGroupDelta(oldControlPlane, newControlPlane)
	if len(addedControlPlanes) > 0 {
		group, err := omniGroupFromMachines(newControlPlane, addedControlPlanes)
		if err != nil {
			return diag.FromErr(err)
		}
		payload["controlPlanesToAdd"] = group
	}
	if len(removedControlPlanes) > 0 {
		payload["controlPlanesToRemove"] = removedControlPlanes
	}

	oldWorker, newWorker := d.GetChange("worker")
	addedWorkers, removedWorkers := omniGroupDelta(oldWorker, newWorker)
	if len(addedWorkers) > 0 {
		group, err := omniGroupFromMachines(newWorker, addedWorkers)
		if err != nil {
			return diag.FromErr(err)
		}
		payload["workersToAdd"] = group
	}
	if len(removedWorkers) > 0 {
		payload["workersToRemove"] = removedWorkers
	}

	updateURL := fmt.Sprintf("%s/omni/%d/cluster/update", client.Endpoint, credentialID)
	if err := doJSON(ctx, client, http.MethodPut, updateURL, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update Omni cluster %s: %w", d.Get("name").(string), err))
	}

	return resourceOmniClusterRead(ctx, d, meta)
}

func resourceOmniClusterDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	credentialID, name, err := omniClusterParts(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	deleteURL := fmt.Sprintf("%s/omni/%d/cluster/delete?name=%s", client.Endpoint, credentialID, url.QueryEscape(name))
	if err := doJSON(ctx, client, http.MethodDelete, deleteURL, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete Omni cluster %s: %w", name, err))
	}

	d.SetId("")
	return nil
}
