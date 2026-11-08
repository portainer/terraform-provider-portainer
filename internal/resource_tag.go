package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceTag() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceTagCreate,
		ReadContext:   resourceTagRead,
		DeleteContext: resourceTagDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the tag. Must be unique within the Portainer instance. Changing this value forces resource recreation.",
			},
		},
	}
}

func resourceTagCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	var created portainerTag
	body := map[string]interface{}{"name": name}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/tags", body, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create tag: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return resourceTagRead(ctx, d, meta)
}

func resourceTagRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Portainer has no GET /tags/{id}, so list and filter.
	tagList, err := listTags(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	for _, tag := range tagList {
		if tag.ID == id {
			if err := d.Set("name", tag.Name); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceTagDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/tags/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete tag: %w", err))
	}

	return nil
}

type portainerTag struct {
	ID   int64  `json:"id"`
	Name string `json:"Name"`
}

// listTags returns every tag (GET /tags).
func listTags(ctx context.Context, client *APIClient) ([]portainerTag, error) {
	var tagList []portainerTag
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/tags", nil, &tagList); err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	return tagList, nil
}
