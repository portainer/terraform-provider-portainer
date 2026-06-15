package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerVolume() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerVolumeRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the Portainer environment (Docker host or Swarm) where the volume is located.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Docker volume to look up.",
			},
			"driver": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Driver used by the Docker volume (e.g., local, nfs, btrfs).",
			},
			"mount_point": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Filesystem path on the Docker host where the volume is mounted.",
			},
		},
	}
}

func dataSourceDockerVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/volumes", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker volumes: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list docker volumes, status %d: %s", resp.StatusCode, string(data)))
	}

	var result struct {
		Volumes []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
		} `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker volume list: %w", err))
	}

	for _, v := range result.Volumes {
		if v.Name == name {
			d.SetId(v.Name) // For volumes, the name is the ID in the API
			if err := d.Set("driver", v.Driver); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("mount_point", v.Mountpoint); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("docker volume %s not found in endpoint %d", name, endpointID))
}
