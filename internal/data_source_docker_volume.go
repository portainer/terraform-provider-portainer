package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDockerVolumeRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"driver": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"mount_point": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceDockerVolumeRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list docker volumes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list docker volumes, status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Volumes []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
		} `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode docker volume list: %w", err)
	}

	for _, v := range result.Volumes {
		if v.Name == name {
			d.SetId(v.Name) // For volumes, the name is the ID in the API
			d.Set("driver", v.Driver)
			d.Set("mount_point", v.Mountpoint)
			return nil
		}
	}

	return fmt.Errorf("docker volume %s not found in endpoint %d", name, endpointID)
}
