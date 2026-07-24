package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerImage() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerImageRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer endpoint where the Docker image resides.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the image (e.g. nginx:latest or nginx)",
			},
		},
	}
}

func dataSourceDockerImageRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	// Ensure name has a tag if it doesn't
	searchName := name
	if !strings.Contains(name, ":") {
		searchName = name + ":latest"
	}

	path := fmt.Sprintf("/endpoints/%d/docker/images/json", endpointID)
	var images []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &images); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker images: %w", err))
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == searchName || tag == name {
				d.SetId(img.ID)
				return nil
			}
		}
	}

	return diag.FromErr(fmt.Errorf("docker image %s not found in endpoint %d", name, endpointID))
}
