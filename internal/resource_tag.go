package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
	"github.com/portainer/client-api-go/v2/pkg/models"
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

	ctx, errBody := withErrorCapture(ctx)
	params := tags.NewTagCreateParams()
	params.SetContext(ctx)
	params.Body = &models.TagsTagCreatePayload{
		Name: &name,
	}

	resp, err := client.Client.Tags.TagCreate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create tag: %w", decorateSDKError(err, errBody)))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceTagRead(ctx, d, meta)
}

func resourceTagRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// SDK does not expose GetTagByID, so we list and filter.
	// This matches the fallback logic of the previous implementation.
	ctx, errBody := withErrorCapture(ctx)
	params := tags.NewTagListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Tags.TagList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list tags: %w", decorateSDKError(err, errBody)))
	}

	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	for _, tag := range resp.Payload {
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

	ctx, errBody := withErrorCapture(ctx)
	params := tags.NewTagDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.Tags.TagDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *tags.TagDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete tag: %w", decorateSDKError(err, errBody)))
	}

	return nil
}
