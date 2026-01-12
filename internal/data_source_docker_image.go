package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerImage() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDockerImageRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the image (e.g. nginx:latest or nginx)",
			},
		},
	}
}

func dataSourceDockerImageRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	// Ensure name has a tag if it doesn't
	searchName := name
	if !strings.Contains(name, ":") {
		searchName = name + ":latest"
	}

	path := fmt.Sprintf("/endpoints/%d/docker/images/json", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list docker images: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list docker images, status %d: %s", resp.StatusCode, string(data))
	}

	var images []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return fmt.Errorf("failed to decode docker image list: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == searchName || tag == name {
				d.SetId(img.ID)
				return nil
			}
		}
	}

	return fmt.Errorf("docker image %s not found in endpoint %d", name, endpointID)
}
