package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerDashboardDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_dashboard",
	schema:     dsDockerDashboardSchema,
	read:       dsDockerDashboardRead,
})

func dsDockerDashboardSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                   schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"containers_healthy":   schema.Int64Attribute{Computed: true, Description: "Number of containers whose healthcheck reports healthy."},
			"containers_running":   schema.Int64Attribute{Computed: true, Description: "Number of running containers."},
			"containers_stopped":   schema.Int64Attribute{Computed: true, Description: "Number of stopped containers."},
			"containers_total":     schema.Int64Attribute{Computed: true, Description: "Total number of containers in the environment."},
			"containers_unhealthy": schema.Int64Attribute{Computed: true, Description: "Number of containers whose healthcheck reports unhealthy. Containers without a healthcheck count towards neither."},
			"environment_id":       schema.Int64Attribute{Required: true, Description: "Environment (endpoint) identifier of the Docker environment to describe."},
			"images_size":          schema.Int64Attribute{Computed: true, Description: "Total size of those images in bytes."},
			"images_total":         schema.Int64Attribute{Computed: true, Description: "Number of images stored on the host."},
			"networks":             schema.Int64Attribute{Computed: true, Description: "Number of networks in the environment."},
			"services":             schema.Int64Attribute{Computed: true, Description: "Number of Swarm services, zero on a standalone Docker host."},
			"stacks":               schema.Int64Attribute{Computed: true, Description: "Number of stacks in the environment."},
			"volumes":              schema.Int64Attribute{Computed: true, Description: "Number of volumes in the environment."},
		},
	}
}

func dsDockerDashboardRead(ctx context.Context, d *dkBag, client *APIClient) error {
	envID := dkInt(d, "environment_id")

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
		return fmt.Errorf("failed to read the Docker dashboard of environment %d: %w", envID, err)
	}

	if err := d.SetFields(map[string]interface{}{
		"containers_total": dash.Containers.Total, "containers_running": dash.Containers.Running,
		"containers_stopped": dash.Containers.Stopped, "containers_healthy": dash.Containers.Healthy,
		"containers_unhealthy": dash.Containers.Unhealthy,
		"images_total":         dash.Images.Total, "images_size": dash.Images.Size,
		"volumes": dash.Volumes, "networks": dash.Networks,
		"services": dash.Services, "stacks": dash.Stacks,
	}); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/dashboard")
	return nil
}
