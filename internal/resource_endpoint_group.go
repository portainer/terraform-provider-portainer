package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoint_groups"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointGroupCreate,
		ReadContext:   resourceEndpointGroupRead,
		DeleteContext: resourceEndpointGroupDelete,
		UpdateContext: resourceEndpointGroupUpdate,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Portainer endpoint group. Must be unique within the Portainer instance.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Human-readable description of the endpoint group displayed in the Portainer UI.",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of Portainer tag identifiers associated with this endpoint group.",
			},
		},
	}
}

func resourceEndpointGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	if existingID, err := findExistingEndpointGroupByName(client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing endpoint group: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEndpointGroupUpdate(ctx, d, meta)
	}

	ctx, errBody := withErrorCapture(ctx)
	params := endpoint_groups.NewPostEndpointGroupsParams()
	params.SetContext(ctx)
	params.Body = &models.EndpointgroupsEndpointGroupCreatePayload{
		Name: &name,
	}

	if v, ok := d.GetOk("description"); ok {
		desc := v.(string)
		params.Body.Description = desc
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int64{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(id.(int)))
		}
		params.Body.TagIDs = tagIDs
	}

	resp, err := client.Client.EndpointGroups.PostEndpointGroups(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create endpoint group: %w", decorateSDKError(err, errBody)))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceEndpointGroupRead(ctx, d, meta)
}

func findExistingEndpointGroupByName(client *APIClient, name string) (int, error) {
	ctx, errBody := withErrorCapture(context.Background())
	params := endpoint_groups.NewEndpointGroupListParams()
	params.SetContext(ctx)
	resp, err := client.Client.EndpointGroups.EndpointGroupList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list endpoint groups: %w", decorateSDKError(err, errBody))
	}

	for _, g := range resp.Payload {
		if g.Name == name {
			return int(g.ID), nil
		}
	}
	return 0, nil
}

func resourceEndpointGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := endpoint_groups.NewGetEndpointGroupsIDParams()
	params.SetContext(ctx)
	params.ID = id

	resp, err := client.Client.EndpointGroups.GetEndpointGroupsID(params, client.AuthInfo)
	if err != nil {
		var notFound *endpoint_groups.GetEndpointGroupsIDNotFound
		if errors.As(err, &notFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read endpoint group: %w", decorateSDKError(err, errBody)))
	}

	if err := d.Set("name", resp.Payload.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", resp.Payload.Description); err != nil {
		return diag.FromErr(err)
	}

	tagIDs := []int{}
	for _, tid := range resp.Payload.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}
	if err := d.Set("tag_ids", tagIDs); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceEndpointGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	ctx, errBody := withErrorCapture(ctx)
	params := endpoint_groups.NewEndpointGroupUpdateParams()
	params.SetContext(ctx)
	params.ID = id
	params.Body = &models.EndpointgroupsEndpointGroupUpdatePayload{
		Name: name,
	}

	if v, ok := d.GetOk("description"); ok {
		params.Body.Description = v.(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int64{}
		for _, tid := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(tid.(int)))
		}
		params.Body.TagIDs = tagIDs
	}

	_, err := client.Client.EndpointGroups.EndpointGroupUpdate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update endpoint group: %w", decorateSDKError(err, errBody)))
	}

	return resourceEndpointGroupRead(ctx, d, meta)
}

func resourceEndpointGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := endpoint_groups.NewEndpointGroupDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.EndpointGroups.EndpointGroupDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *endpoint_groups.EndpointGroupDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete endpoint group: %w", decorateSDKError(err, errBody)))
	}

	return nil
}
