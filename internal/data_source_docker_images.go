package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerImages() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerImagesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Docker environment to query.",
			},
			"with_usage": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to ask Portainer which images are in use by a container. It costs an extra pass over the containers, so it is off by default.",
			},
			// Computed attributes
			"images": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Images stored in the environment.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":        {Type: schema.TypeString, Computed: true, Description: "Image ID, as a sha256 digest."},
						"tags":      {Type: schema.TypeList, Computed: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Repository tags pointing at the image. Empty for a dangling image."},
						"size":      {Type: schema.TypeInt, Computed: true, Description: "Size of the image in bytes."},
						"created":   {Type: schema.TypeInt, Computed: true, Description: "Unix timestamp at which the image was created."},
						"node_name": {Type: schema.TypeString, Computed: true, Description: "Swarm node holding the image, empty on a standalone host."},
						"used":      {Type: schema.TypeBool, Computed: true, Description: "Whether a container uses the image. Always false unless `with_usage` is set."},
					},
				},
			},
			"total_size": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Combined size of every listed image, in bytes.",
			},
			"unused_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of images no container uses. Only meaningful when `with_usage` is set.",
			},
		},
	}
}

func dataSourceDockerImagesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	withUsage := d.Get("with_usage").(bool)

	target := fmt.Sprintf("%s/docker/%d/images", client.Endpoint, envID)
	if withUsage {
		target += "?withUsage=true"
	}

	var list []struct {
		ID       string   `json:"id"`
		Tags     []string `json:"tags"`
		Size     int      `json:"size"`
		Created  int      `json:"created"`
		NodeName string   `json:"nodeName"`
		Used     bool     `json:"used"`
	}
	if err := doJSON(ctx, client, http.MethodGet, target, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the images of environment %d: %w", envID, err))
	}

	images := make([]map[string]interface{}, len(list))
	totalSize, unused := 0, 0
	for i, img := range list {
		tags := img.Tags
		if tags == nil {
			tags = []string{}
		}
		totalSize += img.Size
		if withUsage && !img.Used {
			unused++
		}
		images[i] = map[string]interface{}{
			"id": img.ID, "tags": tags, "size": img.Size,
			"created": img.Created, "node_name": img.NodeName, "used": img.Used,
		}
	}

	if err := setFields(d, map[string]interface{}{
		"images": images, "total_size": totalSize, "unused_count": unused,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/images")
	return nil
}
