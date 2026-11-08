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

	if existingID, err := findExistingEndpointGroupByName(ctx, client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing endpoint group: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEndpointGroupUpdate(ctx, d, meta)
	}

	body := struct {
		Name                string  `json:"name"`
		Description         string  `json:"description,omitempty"`
		AssociatedEndpoints []int64 `json:"associatedEndpoints"`
		TagIDs              []int64 `json:"tagIDs"`
	}{Name: name}

	if v, ok := d.GetOk("description"); ok {
		body.Description = v.(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int64{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(id.(int)))
		}
		body.TagIDs = tagIDs
	}

	var created portainerEndpointGroup
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/endpoint_groups", body, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create endpoint group: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return resourceEndpointGroupRead(ctx, d, meta)
}

func findExistingEndpointGroupByName(ctx context.Context, client *APIClient, name string) (int, error) {
	groups, err := listEndpointGroups(ctx, client)
	if err != nil {
		return 0, err
	}

	for _, g := range groups {
		if g.Name == name {
			return int(g.ID), nil
		}
	}
	return 0, nil
}

func resourceEndpointGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	var group portainerEndpointGroup
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, id), nil, &group); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read endpoint group: %w", err))
	}

	if err := d.Set("name", group.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", group.Description); err != nil {
		return diag.FromErr(err)
	}

	tagIDs := []int{}
	for _, tid := range group.TagIds {
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

	body := struct {
		Name        string  `json:"name,omitempty"`
		Description string  `json:"description,omitempty"`
		TagIDs      []int64 `json:"tagIDs"`
	}{Name: name}

	if v, ok := d.GetOk("description"); ok {
		body.Description = v.(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := make([]int64, 0, len(v.([]interface{})))
		for _, tid := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(tid.(int)))
		}
		body.TagIDs = tagIDs
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, id), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update endpoint group: %w", err))
	}

	return resourceEndpointGroupRead(ctx, d, meta)
}

func resourceEndpointGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete endpoint group: %w", err))
	}

	return nil
}

type portainerEndpointGroup struct {
	ID          int64   `json:"Id"`
	Name        string  `json:"Name"`
	Description string  `json:"Description"`
	TagIds      []int64 `json:"TagIds"`
}

// listEndpointGroups returns every environment group (GET /endpoint_groups).
func listEndpointGroups(ctx context.Context, client *APIClient) ([]portainerEndpointGroup, error) {
	var groups []portainerEndpointGroup
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoint_groups", nil, &groups); err != nil {
		return nil, fmt.Errorf("failed to list endpoint groups: %w", err)
	}
	return groups, nil
}
