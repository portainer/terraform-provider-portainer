package internal

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DockerVolumeSpec struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver,omitempty"`
	DriverOpts map[string]string `json:"DriverOpts,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
}

func resourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerVolumeCreate,
		Read:   resourceDockerVolumeRead,
		Delete: resourceDockerVolumeDelete,
		Update: nil,
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
	// Stateless
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
