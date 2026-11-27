package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var newDockerImagesDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_images",
	schema:     dsDockerImagesSchema,
	read:       dsDockerImagesRead,
})

func dsDockerImagesSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"environment_id": schema.Int64Attribute{Required: true, Description: "Environment (endpoint) identifier of the Docker environment to query."},
			"images":         schema.ListAttribute{ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"created": types.Int64Type, "id": types.StringType, "node_name": types.StringType, "size": types.Int64Type, "tags": types.ListType{ElemType: types.StringType}, "used": types.BoolType}}, Computed: true, Description: "Images stored in the environment."},
			"total_size":     schema.Int64Attribute{Computed: true, Description: "Combined size of every listed image, in bytes."},
			"unused_count":   schema.Int64Attribute{Computed: true, Description: "Number of images no container uses. Only meaningful when `with_usage` is set."},
			"with_usage":     schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to ask Portainer which images are in use by a container. It costs an extra pass over the containers, so it is off by default."},
		},
	}
}

func dsDockerImagesRead(ctx context.Context, d *dkBag, client *APIClient) error {
	envID := dkInt(d, "environment_id")
	withUsage := dkBool(d, "with_usage")

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
		return fmt.Errorf("failed to list the images of environment %d: %w", envID, err)
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

	if err := d.SetFields(map[string]interface{}{
		"images": images, "total_size": totalSize, "unused_count": unused,
	}); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/images")
	return nil
}
