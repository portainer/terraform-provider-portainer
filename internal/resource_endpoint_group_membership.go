package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointGroupMembership() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointGroupMembershipCreate,
		ReadContext:   resourceEndpointGroupMembershipRead,
		DeleteContext: resourceEndpointGroupMembershipDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_group_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the environment group the environment is added to.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the environment to place in the group.",
			},
		},
	}
}

func resourceEndpointGroupMembershipCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	groupID := d.Get("endpoint_group_id").(int)
	endpointID := d.Get("endpoint_id").(int)

	url := fmt.Sprintf("%s/endpoint_groups/%d/endpoints/%d", client.Endpoint, groupID, endpointID)
	if err := doJSON(ctx, client, http.MethodPut, url, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to add environment %d to group %d: %w", endpointID, groupID, err))
	}

	d.SetId(fmt.Sprintf("%d/%d", groupID, endpointID))
	return resourceEndpointGroupMembershipRead(ctx, d, meta)
}

func resourceEndpointGroupMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	groupID := d.Get("endpoint_group_id").(int)
	endpointID := d.Get("endpoint_id").(int)

	// Membership is a property of the environment, not a resource of its own:
	// endpoint.GroupId is the single source of truth, and moving an environment
	// to another group silently removes it from this one.
	var endpoint struct {
		GroupID int `json:"GroupId"`
	}
	url := fmt.Sprintf("%s/endpoints/%d", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &endpoint); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read environment %d: %w", endpointID, err))
	}

	if endpoint.GroupID != groupID {
		d.SetId("")
		return nil
	}

	return nil
}

func resourceEndpointGroupMembershipDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	groupID := d.Get("endpoint_group_id").(int)
	endpointID := d.Get("endpoint_id").(int)

	// Portainer moves the environment back to the Unassigned group (ID 1); there
	// is no state in which an environment belongs to no group at all.
	url := fmt.Sprintf("%s/endpoint_groups/%d/endpoints/%d", client.Endpoint, groupID, endpointID)
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to remove environment %d from group %d: %w", endpointID, groupID, err))
	}

	d.SetId("")
	return nil
}
