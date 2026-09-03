package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerDashboard() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerDashboardRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Docker environment to describe.",
			},
			// Computed attributes
			"containers_total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total number of containers in the environment.",
			},
			"containers_running": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of running containers.",
			},
			"containers_stopped": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of stopped containers.",
			},
			"containers_healthy": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of containers whose healthcheck reports healthy.",
			},
			"containers_unhealthy": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of containers whose healthcheck reports unhealthy. Containers without a healthcheck count towards neither.",
			},
			"images_total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of images stored on the host.",
			},
			"images_size": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total size of those images in bytes.",
			},
			"volumes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of volumes in the environment.",
			},
			"networks": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of networks in the environment.",
			},
			"services": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Swarm services, zero on a standalone Docker host.",
			},
			"stacks": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of stacks in the environment.",
			},
		},
	}
}

func dataSourceDockerDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var dash struct {
		Containers struct {
			Total     int `json:"total"`
			Running   int `json:"running"`
			Stopped   int `json:"stopped"`
			Healthy   int `json:"healthy"`
			Unhealthy int `json:"unhealthy"`
		} `json:"containers"`
		Images struct {
			Total int `json:"total"`
			Size  int `json:"size"`
		} `json:"images"`
		Volumes  int `json:"volumes"`
		Networks int `json:"networks"`
		Services int `json:"services"`
		Stacks   int `json:"stacks"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/docker/%d/dashboard", client.Endpoint, envID), nil, &dash); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Docker dashboard of environment %d: %w", envID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"containers_total": dash.Containers.Total, "containers_running": dash.Containers.Running,
		"containers_stopped": dash.Containers.Stopped, "containers_healthy": dash.Containers.Healthy,
		"containers_unhealthy": dash.Containers.Unhealthy,
		"images_total":         dash.Images.Total, "images_size": dash.Images.Size,
		"volumes": dash.Volumes, "networks": dash.Networks,
		"services": dash.Services, "stacks": dash.Stacks,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/dashboard")
	return nil
}
