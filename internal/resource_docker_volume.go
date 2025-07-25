package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

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

func resourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerVolumeCreate,
		Read:   resourceDockerVolumeRead,
		Delete: resourceDockerVolumeDelete,
		Update: nil,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"driver": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "local",
			},
			"driver_opts": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"cluster_volume_spec": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group": {Type: schema.TypeString, Optional: true},
						"access_mode": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"scope":        {Type: schema.TypeString, Optional: true},
									"sharing":      {Type: schema.TypeString, Optional: true},
									"mount_volume": {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}},
								},
							},
						},
						"secrets": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"key":    {Type: schema.TypeString, Required: true},
									"secret": {Type: schema.TypeString, Required: true},
								},
							},
						},
						"accessibility_requirements": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"requisite": {Type: schema.TypeList, Optional: true, Elem: &schema.Resource{Schema: map[string]*schema.Schema{"property1": {Type: schema.TypeString, Optional: true}, "property2": {Type: schema.TypeString, Optional: true}}}},
									"preferred": {Type: schema.TypeList, Optional: true, Elem: &schema.Resource{Schema: map[string]*schema.Schema{"property1": {Type: schema.TypeString, Optional: true}, "property2": {Type: schema.TypeString, Optional: true}}}},
								},
							},
						},
						"capacity_range": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"required_bytes": {Type: schema.TypeInt, Optional: true},
									"limit_bytes":    {Type: schema.TypeInt, Optional: true},
								},
							},
						},
						"availability": {Type: schema.TypeString, Optional: true},
					},
				},
			},
		},
	}
}

func resourceDockerVolumeCreate(d *schema.ResourceData, meta interface{}) error {
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
	resp, err := client.DoRequest(http.MethodPost, path, nil, volume)
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to create volume, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	d.SetId(fmt.Sprintf("%d-%s", endpointID, volume.Name))
	return nil
}

func resourceDockerVolumeRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes/%s", endpointID, url.PathEscape(name))
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read docker volume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read volume: %s", string(body))
	}

	var result struct {
		Name       string            `json:"Name"`
		Driver     string            `json:"Driver"`
		Labels     map[string]string `json:"Labels"`
		Options    map[string]string `json:"Options"`
		Mountpoint string            `json:"Mountpoint"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode volume: %w", err)
	}

	d.Set("name", result.Name)
	d.Set("driver", result.Driver)
	d.Set("labels", result.Labels)
	d.Set("driver_opts", result.Options)
	d.SetId(fmt.Sprintf("%d-%s", endpointID, name))
	return nil
}

func resourceDockerVolumeDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes/%s", endpointID, url.PathEscape(name))
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete volume, status code: %d, body: %s", resp.StatusCode, string(body))
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
