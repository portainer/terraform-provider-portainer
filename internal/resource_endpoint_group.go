package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoint_groups"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointGroupCreate,
		Read:   resourceEndpointGroupRead,
		Delete: resourceEndpointGroupDelete,
		Update: resourceEndpointGroupUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"tag_ids": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func resourceEndpointGroupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	if existingID, err := findExistingEndpointGroupByName(client, name); err != nil {
		return fmt.Errorf("failed to check for existing endpoint group: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEndpointGroupUpdate(d, meta)
	}

	params := endpoint_groups.NewPostEndpointGroupsParams()
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
		return fmt.Errorf("failed to create endpoint group: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceEndpointGroupRead(d, meta)
}

func findExistingEndpointGroupByName(client *APIClient, name string) (int, error) {
	params := endpoint_groups.NewEndpointGroupListParams()
	resp, err := client.Client.EndpointGroups.EndpointGroupList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list endpoint groups: %w", err)
	}

	for _, g := range resp.Payload {
		if g.Name == name {
			return int(g.ID), nil
		}
	}
	return 0, nil
}

func resourceEndpointGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := endpoint_groups.NewGetEndpointGroupsIDParams()
	params.ID = id

	resp, err := client.Client.EndpointGroups.GetEndpointGroupsID(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*endpoint_groups.GetEndpointGroupsIDNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read endpoint group: %w", err)
	}

	d.Set("name", resp.Payload.Name)
	d.Set("description", resp.Payload.Description)

	tagIDs := []int{}
	for _, tid := range resp.Payload.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}
	d.Set("tag_ids", tagIDs)

	return nil
}

func resourceEndpointGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	params := endpoint_groups.NewEndpointGroupUpdateParams()
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
		return fmt.Errorf("failed to update endpoint group: %w", err)
	}

	return resourceEndpointGroupRead(d, meta)
}

func resourceEndpointGroupDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := endpoint_groups.NewEndpointGroupDeleteParams()
	params.ID = id

	_, err := client.Client.EndpointGroups.EndpointGroupDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*endpoint_groups.EndpointGroupDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete endpoint group: %w", err)
	}

	return nil
}
