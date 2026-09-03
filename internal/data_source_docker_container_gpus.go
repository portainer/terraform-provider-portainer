package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerContainerGpus() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerContainerGpusRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Docker environment the container runs in.",
			},
			"container_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the container to inspect.",
			},
			// Computed attributes
			"gpus": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "GPUs assigned to the container, as Portainer reports them. Empty when the container has none.",
			},
		},
	}
}

func dataSourceDockerContainerGpusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	containerID := d.Get("container_id").(string)

	var result struct {
		Gpus string `json:"gpus"`
	}
	url := fmt.Sprintf("%s/docker/%d/containers/%s/gpus", client.Endpoint, envID, containerID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the GPUs of container %s: %w", containerID, err))
	}

	if err := d.Set("gpus", result.Gpus); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/gpus", envID, containerID))
	return nil
}
