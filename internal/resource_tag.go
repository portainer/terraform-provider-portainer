package internal

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceTag() *schema.Resource {
	return &schema.Resource{
		Create: resourceTagCreate,
		Read:   resourceTagRead,
		Delete: resourceTagDelete,
		Update: nil,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},
		},
	}
}

func resourceTagCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	params := tags.NewTagCreateParams()
	params.Body = &models.TagsTagCreatePayload{
		Name: &name,
	}

	resp, err := client.Client.Tags.TagCreate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceTagRead(d, meta)
}

func resourceTagRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// SDK does not expose GetTagByID, so we list and filter.
	// This matches the fallback logic of the previous implementation.
	params := tags.NewTagListParams()
	resp, err := client.Client.Tags.TagList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	for _, tag := range resp.Payload {
		if tag.ID == id {
			d.Set("name", tag.Name)
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceTagDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := tags.NewTagDeleteParams()
	params.ID = id

	_, err := client.Client.Tags.TagDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *tags.TagDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return nil
}
