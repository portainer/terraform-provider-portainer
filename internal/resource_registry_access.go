package internal

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoints"
	"github.com/portainer/client-api-go/v2/pkg/client/registries"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

var ErrRegistryNotFound = errors.New("registry not found")

func resourceRegistryAccess() *schema.Resource {
	return &schema.Resource{
		Create: resourceRegistryAccessCreate,
		Read:   resourceRegistryAccessRead,
		Update: resourceRegistryAccessUpdate,
		Delete: resourceRegistryAccessDelete,

		Schema: map[string]*schema.Schema{
			"registry_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"team_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"user_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"role_id": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
		},
	}
}

func getRegistryPolicies(client *APIClient, registryID int, endpointID int) (*models.PortainerRegistryAccessPolicies, error) {
	params := registries.NewRegistryInspectParams()
	params.ID = int64(registryID)

	resp, err := client.Client.Registries.RegistryInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*registries.RegistryInspectNotFound); ok {
			return nil, ErrRegistryNotFound
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	eidStr := strconv.Itoa(endpointID)
	policies, ok := resp.Payload.RegistryAccesses[eidStr]
	if !ok {
		return &models.PortainerRegistryAccessPolicies{
			UserAccessPolicies: make(models.PortainerUserAccessPolicies),
			TeamAccessPolicies: make(models.PortainerTeamAccessPolicies),
		}, nil
	}

	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(models.PortainerUserAccessPolicies)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(models.PortainerTeamAccessPolicies)
	}

	return &policies, nil
}

func resourceRegistryAccessCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")
	roleID := int64(d.Get("role_id").(int))

	if !hasTeam && !hasUser {
		return fmt.Errorf("either team_id or user_id must be provided")
	}

	policies, err := getRegistryPolicies(client, registryID, endpointID)
	if err != nil {
		return err
	}

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		policies.TeamAccessPolicies[tidStr] = models.PortainerAccessPolicy{RoleID: roleID}
	}
	if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		policies.UserAccessPolicies[uidStr] = models.PortainerAccessPolicy{RoleID: roleID}
	}

	params := endpoints.NewEndpointRegistryAccessParams()
	params.ID = int64(endpointID)
	params.RegistryID = int64(registryID)
	params.Body = &models.EndpointsRegistryAccessPayload{
		UserAccessPolicies: policies.UserAccessPolicies,
		TeamAccessPolicies: policies.TeamAccessPolicies,
		Namespaces:         policies.Namespaces,
	}

	_, err = client.Client.Endpoints.EndpointRegistryAccess(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update registry access: %w", err)
	}

	id := fmt.Sprintf("%d/%d/", registryID, endpointID)
	if hasTeam {
		id += fmt.Sprintf("team/%d", teamID.(int))
	} else {
		id += fmt.Sprintf("user/%d", userID.(int))
	}
	d.SetId(id)

	return resourceRegistryAccessRead(d, meta)
}

func resourceRegistryAccessRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getRegistryPolicies(client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}

	found := false
	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			d.Set("role_id", int(p.RoleID))
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			d.Set("role_id", int(p.RoleID))
			found = true
		}
	}

	if !found {
		d.SetId("")
	}

	return nil
}

func resourceRegistryAccessUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourceRegistryAccessCreate(d, meta)
}

func resourceRegistryAccessDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getRegistryPolicies(client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return nil
		}
		return err
	}

	if hasTeam {
		delete(policies.TeamAccessPolicies, strconv.Itoa(teamID.(int)))
	}
	if hasUser {
		delete(policies.UserAccessPolicies, strconv.Itoa(userID.(int)))
	}

	params := endpoints.NewEndpointRegistryAccessParams()
	params.ID = int64(endpointID)
	params.RegistryID = int64(registryID)
	params.Body = &models.EndpointsRegistryAccessPayload{
		UserAccessPolicies: policies.UserAccessPolicies,
		TeamAccessPolicies: policies.TeamAccessPolicies,
		Namespaces:         policies.Namespaces,
	}

	_, err = client.Client.Endpoints.EndpointRegistryAccess(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*endpoints.EndpointRegistryAccessNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete registry access: %w", err)
	}

	return nil
}
