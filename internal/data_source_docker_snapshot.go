package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceDockerSnapshot() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerSnapshotRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Docker environment whose snapshot is read.",
			},
			// Computed attributes
			"snapshot": {
				Type:     schema.TypeString,
				Computed: true,
				// Portainer's own specification gives this response no fields
				// at all, so there is nothing to flatten it into that would
				// stay accurate.
				Description: "The latest snapshot as Portainer returns it, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_docker_snapshot_containers` for a typed view of the containers in it.",
			},
		},
	}
}

func dataSourceDockerSnapshotRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	snapshotURL := fmt.Sprintf("%s/docker/%d/snapshot", client.Endpoint, endpointID)
	body, err := apiGETRaw(ctx, client, snapshotURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the snapshot of environment %d: %w", endpointID, err))
	}

	if err := setFields(d, map[string]interface{}{"snapshot": string(body)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-docker-snapshot-%d", endpointID))
	return nil
}

// dockerSnapshotContainer is the container shape both snapshot endpoints
// return. Only the fields worth acting on are decoded; the rest stays in the
// raw response each data source also exposes.
type dockerSnapshotContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

func (c dockerSnapshotContainer) fields() map[string]interface{} {
	ports := make([]interface{}, 0, len(c.Ports))
	for _, port := range c.Ports {
		ports = append(ports, map[string]interface{}{
			"ip": port.IP, "private_port": port.PrivatePort,
			"public_port": port.PublicPort, "type": port.Type,
		})
	}
	return map[string]interface{}{
		"id": c.ID, "names": c.Names, "image": c.Image, "image_id": c.ImageID,
		"command": c.Command, "created": int(c.Created), "state": c.State,
		"status": c.Status, "labels": stringMap(c.Labels), "ports": ports,
	}
}

// dockerSnapshotContainerSchema is written out in each data source below
// rather than shared, because docsdriftlint and schemalint read the schemas
// statically.

func dataSourceDockerSnapshotContainers() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerSnapshotContainersRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Docker environment whose snapshot is read.",
			},
			"edge_stack_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Only list the containers belonging to this edge stack.",
			},
			// Computed attributes
			"containers": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The containers in the latest snapshot. This is what Portainer last saw, not a live query, so it can lag behind the environment.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Container identifier.",
						},
						"names": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Names of the container, as Docker reports them with a leading slash.",
						},
						"image": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Image the container runs.",
						},
						"image_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Identifier of that image.",
						},
						"command": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Command the container runs.",
						},
						"created": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Unix timestamp the container was created at.",
						},
						"state": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "State of the container, for example `running` or `exited`.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Human-readable status, for example `Up 3 hours`.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels on the container.",
						},
						"ports": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Ports the container exposes.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"ip": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Host address the port is bound to.",
									},
									"private_port": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Port inside the container.",
									},
									"public_port": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Port on the host, zero when the port is not published.",
									},
									"type": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Protocol of the port.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dataSourceDockerSnapshotContainersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	listURL := fmt.Sprintf("%s/docker/%d/snapshot/containers", client.Endpoint, endpointID)
	if v, ok := d.GetOk("edge_stack_id"); ok && v.(int) != 0 {
		listURL += fmt.Sprintf("?edgeStackId=%d", v.(int))
	}

	body, err := apiGETRaw(ctx, client, listURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the snapshot containers of environment %d: %w", endpointID, err))
	}

	// Portainer's specification types this response as a single container even
	// though the endpoint lists them, so both shapes are accepted rather than
	// trusting the document over the endpoint's name.
	var containers []dockerSnapshotContainer
	if err := json.Unmarshal(body, &containers); err != nil {
		var single dockerSnapshotContainer
		if err := json.Unmarshal(body, &single); err != nil {
			return diag.FromErr(fmt.Errorf("failed to parse the snapshot containers of environment %d: %w", endpointID, err))
		}
		containers = []dockerSnapshotContainer{single}
	}

	entries := make([]interface{}, 0, len(containers))
	for _, container := range containers {
		entries = append(entries, container.fields())
	}

	if err := setFields(d, map[string]interface{}{"containers": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-docker-snapshot-containers-%d", endpointID))
	return nil
}

func dataSourceDockerSnapshotContainer() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerSnapshotContainerRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Docker environment whose snapshot is read.",
			},
			"container_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the container to read from the snapshot.",
			},
			// Computed attributes
			"names": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Names of the container, as Docker reports them with a leading slash.",
			},
			"image": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Image the container runs.",
			},
			"image_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Identifier of that image.",
			},
			"command": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Command the container runs.",
			},
			"created": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp the container was created at.",
			},
			"state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "State of the container, for example `running` or `exited`.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Human-readable status, for example `Up 3 hours`.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Labels on the container.",
			},
			"details": {
				Type:     schema.TypeString,
				Computed: true,
				// Mounts, network settings and the host configuration nest
				// deeply and follow Docker's own schema rather than Portainer's.
				Description: "The whole snapshot entry as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the mounts, networks and host configuration the attributes above do not cover.",
			},
		},
	}
}

func dataSourceDockerSnapshotContainerRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	containerID := d.Get("container_id").(string)

	containerURL := fmt.Sprintf("%s/docker/%d/snapshot/containers/%s",
		client.Endpoint, endpointID, url.PathEscape(containerID))
	body, err := apiGETRaw(ctx, client, containerURL)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read container %s from the snapshot of environment %d: %w", containerID, endpointID, err))
	}

	var container dockerSnapshotContainer
	if err := json.Unmarshal(body, &container); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse container %s from the snapshot: %w", containerID, err))
	}

	fields := container.fields()
	// id and ports are not top-level attributes here: the identifier is an
	// argument, and the port list belongs to the listing data source.
	delete(fields, "id")
	delete(fields, "ports")
	fields["details"] = string(body)

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-docker-snapshot-container-%d-%s", endpointID, containerID))
	return nil
}

// dataSourceImageStatus reports whether the image behind a container, a
// service or a whole stack has been superseded in its registry. Portainer has
// three endpoints for it, one per subject, which is what `kind` selects.
func dataSourceImageStatus() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceImageStatusRead,

		Schema: map[string]*schema.Schema{
			"kind": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"container", "service", "stack"}, false),
				Description:  "What to check: `container`, `service` or `stack`.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the environment. Required for `container` and `service`; a stack is addressed by its own identifier instead.",
			},
			"resource_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Identifier of the container, service or stack to check.",
			},
			"refresh": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to re-check the registry rather than answer from Portainer's cache. A refresh costs a registry round trip per image.",
			},
			// Computed attributes
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "What Portainer found: `updated`, `outdated`, `skipped`, `processing`, `preparing` or `error`. Only `outdated` means a newer image is published.",
			},
			"message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Explanation of the status, empty when there is nothing to say.",
			},
		},
	}
}

func dataSourceImageStatusRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	kind := d.Get("kind").(string)
	resourceID := d.Get("resource_id").(string)
	endpointID := d.Get("endpoint_id").(int)

	var statusURL string
	switch kind {
	case "container":
		if endpointID == 0 {
			return diag.FromErr(fmt.Errorf("endpoint_id is required when kind is %q", kind))
		}
		statusURL = fmt.Sprintf("%s/docker/%d/containers/%s/image_status",
			client.Endpoint, endpointID, url.PathEscape(resourceID))
	case "service":
		if endpointID == 0 {
			return diag.FromErr(fmt.Errorf("endpoint_id is required when kind is %q", kind))
		}
		statusURL = fmt.Sprintf("%s/docker/%d/services/%s/image_status",
			client.Endpoint, endpointID, url.PathEscape(resourceID))
	default:
		// A stack is addressed by its own identifier, with no environment in
		// the path.
		statusURL = fmt.Sprintf("%s/stacks/%s/images_status", client.Endpoint, url.PathEscape(resourceID))
	}
	if d.Get("refresh").(bool) {
		statusURL += "?refresh=true"
	}

	var response struct {
		Status  string `json:"Status"`
		Message string `json:"Message"`
	}
	if err := doJSON(ctx, client, http.MethodGet, statusURL, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the image status of %s %s: %w", kind, resourceID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"status":  response.Status,
		"message": response.Message,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-image-status-%s-%d-%s", kind, endpointID, resourceID))
	return nil
}
