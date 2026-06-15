package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DockerVolumeSpec struct {
	Name              string             `json:"Name"`
	Driver            string             `json:"Driver,omitempty"`
	DriverOpts        map[string]string  `json:"DriverOpts,omitempty"`
	Labels            map[string]string  `json:"Labels,omitempty"`
	ClusterVolumeSpec *ClusterVolumeSpec `json:"ClusterVolumeSpec,omitempty"`
}

type ClusterVolumeSpec struct {
	Group                     string                     `json:"Group,omitempty"`
	AccessMode                *AccessMode                `json:"AccessMode,omitempty"`
	Secrets                   []ClusterVolumeSecret      `json:"Secrets,omitempty"`
	AccessibilityRequirements *AccessibilityRequirements `json:"AccessibilityRequirements,omitempty"`
	CapacityRange             *CapacityRange             `json:"CapacityRange,omitempty"`
	Availability              string                     `json:"Availability,omitempty"`
}

type AccessMode struct {
	Scope       string            `json:"Scope,omitempty"`
	Sharing     string            `json:"Sharing,omitempty"`
	MountVolume map[string]string `json:"MountVolume,omitempty"`
}

type ClusterVolumeSecret struct {
	Key    string `json:"Key"`
	Secret string `json:"Secret"`
}

type AccessibilityRequirements struct {
	Requisite []map[string]string `json:"Requisite,omitempty"`
	Preferred []map[string]string `json:"Preferred,omitempty"`
}

type CapacityRange struct {
	RequiredBytes int64 `json:"RequiredBytes,omitempty"`
	LimitBytes    int64 `json:"LimitBytes,omitempty"`
}

type dockerVolumeCreateResponse struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Labels     map[string]string `json:"Labels"`
	Options    map[string]string `json:"Options"`
	Mountpoint string            `json:"Mountpoint"`
	Portainer  struct {
		ResourceControl struct {
			Id int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func resourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerVolumeCreate,
		ReadContext:   resourceDockerVolumeRead,
		DeleteContext: resourceDockerVolumeDelete,
		UpdateContext: nil,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				// ID in format: <endpoint_id>-<volume_name>
				id := d.Id()
				var endpointID int
				var name string
				n, err := fmt.Sscanf(id, "%d-%s", &endpointID, &name)
				if err != nil || n != 2 {
					return nil, fmt.Errorf("invalid import ID format. Expected '<endpoint_id>-<volume_name>'")
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				if err := d.Set("name", name); err != nil {
					return nil, err
				}
				d.SetId(fmt.Sprintf("%d-%s", endpointID, name))
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the Portainer environment (Docker host or Swarm) where the volume is created.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Docker volume.",
			},
			"driver": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     "local",
				Description: "Driver used by the Docker volume (e.g., local, nfs, btrfs).",
			},
			"driver_opts": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Driver-specific options passed to the Docker volume driver.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Key/value labels attached to the Docker volume.",
			},
			"cluster_volume_spec": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Description: "Specification for a Swarm cluster volume (CSI-backed volume managed by Docker Swarm).",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group": {Type: schema.TypeString, Optional: true, Description: "Group name used to associate related cluster volumes."},
						"access_mode": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Access mode describing how the cluster volume may be mounted by tasks.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"scope":        {Type: schema.TypeString, Optional: true, Description: "Scope of the cluster volume access (e.g., single or multi node)."},
									"sharing":      {Type: schema.TypeString, Optional: true, Description: "Sharing mode for the cluster volume (e.g., none, readonly, onewriter, all)."},
									"mount_volume": {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Filesystem mount options applied when the cluster volume is attached."},
								},
							},
						},
						"secrets": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Secrets passed to the CSI plugin when provisioning the cluster volume.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"key":    {Type: schema.TypeString, Required: true, Description: "Key used by the CSI plugin to identify the secret value."},
									"secret": {Type: schema.TypeString, Required: true, Description: "Name of the Docker Swarm secret providing the value."},
								},
							},
						},
						"accessibility_requirements": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Topology constraints describing where the cluster volume can be accessed.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"requisite": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Topologies in which the cluster volume must be accessible.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"property1": {Type: schema.TypeString, Optional: true, Description: "First topology key/value pair describing a required location."},
												"property2": {Type: schema.TypeString, Optional: true, Description: "Second topology key/value pair describing a required location."},
											},
										},
									},
									"preferred": {
										Type:        schema.TypeList,
										Optional:    true,
										Description: "Topologies in which the cluster volume should preferably be accessible.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"property1": {Type: schema.TypeString, Optional: true, Description: "First topology key/value pair describing a preferred location."},
												"property2": {Type: schema.TypeString, Optional: true, Description: "Second topology key/value pair describing a preferred location."},
											},
										},
									},
								},
							},
						},
						"capacity_range": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Capacity bounds requested for the cluster volume.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"required_bytes": {Type: schema.TypeInt, Optional: true, Description: "Minimum required size of the cluster volume in bytes."},
									"limit_bytes":    {Type: schema.TypeInt, Optional: true, Description: "Maximum allowed size of the cluster volume in bytes."},
								},
							},
						},
						"availability": {Type: schema.TypeString, Optional: true, Description: "Availability of the cluster volume (active, pause, or drain)."},
					},
				},
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the Portainer resource control associated with this Docker volume.",
			},
		},
	}
}

func resourceDockerVolumeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	volume := DockerVolumeSpec{
		Name:       d.Get("name").(string),
		Driver:     d.Get("driver").(string),
		DriverOpts: convertMapString(d.Get("driver_opts").(map[string]interface{})),
		Labels:     convertMapString(d.Get("labels").(map[string]interface{})),
	}

	if rawSpec, ok := d.GetOk("cluster_volume_spec"); ok {
		specList := rawSpec.([]interface{})
		if len(specList) > 0 && specList[0] != nil {
			volume.ClusterVolumeSpec = expandClusterVolumeSpec(specList[0].(map[string]interface{}))
		}
	}

	endpointID := d.Get("endpoint_id").(int)
	path := fmt.Sprintf("/endpoints/%d/docker/volumes/create", endpointID)

	var response dockerVolumeCreateResponse

	resp, err := client.DoRequest(http.MethodPost, path, nil, volume)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create volume: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create volume, status code: %d, body: %s", resp.StatusCode, string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode create volume response: %w", err))
	}

	name := response.Name
	if name == "" {
		name = volume.Name
	}

	d.SetId(fmt.Sprintf("%d-%s", endpointID, name))

	if response.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", response.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes/%s", endpointID, url.PathEscape(name))
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read docker volume: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	} else if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read volume: %s", string(body)))
	}

	var result struct {
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Labels     map[string]string `json:"Labels"`
		Options    map[string]string `json:"Options"`
		Mountpoint string            `json:"Mountpoint"`
		Portainer  struct {
			ResourceControl struct {
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode volume: %w", err))
	}

	_ = d.Set("name", result.Name)
	_ = d.Set("driver", result.Driver)
	_ = d.Set("labels", result.Labels)
	_ = d.Set("driver_opts", result.Options)

	d.SetId(fmt.Sprintf("%d-%s", endpointID, result.Name))

	if result.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", result.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerVolumeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes/%s", endpointID, url.PathEscape(name))
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete volume: %w", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return diag.FromErr(fmt.Errorf("failed to delete volume, status code: %d, body: %s", resp.StatusCode, string(body)))
	}

	d.SetId("")
	return nil
}

func convertMapString(in map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

func expandClusterVolumeSpec(m map[string]interface{}) *ClusterVolumeSpec {
	return &ClusterVolumeSpec{
		Group:        m["group"].(string),
		Availability: m["availability"].(string),
	}
}
