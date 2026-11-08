package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var ErrRegistryNotFound = errors.New("registry not found")

func resourceRegistryAccess() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRegistryAccessCreate,
		ReadContext:   resourceRegistryAccessRead,
		UpdateContext: resourceRegistryAccessUpdate,
		DeleteContext: resourceRegistryAccessDelete,
		Importer: &schema.ResourceImporter{
			StateContext: importRegistryAccess,
		},

		Schema: map[string]*schema.Schema{
			"registry_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer registry whose access policy is being managed.",
			},
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer endpoint to which the registry access policy applies.",
			},
			"team_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the team granted access to the registry. Mutually exclusive with `user_id`.",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the user granted access to the registry. Mutually exclusive with `team_id`.",
			},
			"role_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Role identifier assigned to the user or team for the registry on the given endpoint.",
			},
		},
	}
}

// registryAccessPolicies is a registry's access on one environment, as
// Portainer reports it in Registry.RegistryAccesses.
type registryAccessPolicies struct {
	UserAccessPolicies map[string]accessPolicy `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]accessPolicy `json:"TeamAccessPolicies"`
	Namespaces         []string                `json:"Namespaces"`
}

type accessPolicy struct {
	RoleID int64 `json:"RoleId,omitempty"`
}

// registryAccessPayload is the PUT /endpoints/{id}/registries/{registryId}
// body.
type registryAccessPayload struct {
	UserAccessPolicies map[string]accessPolicy `json:"userAccessPolicies,omitempty"`
	TeamAccessPolicies map[string]accessPolicy `json:"teamAccessPolicies,omitempty"`
	Namespaces         []string                `json:"namespaces"`
}

func getRegistryPolicies(ctx context.Context, client *APIClient, registryID int, endpointID int) (*registryAccessPolicies, error) {
	registry, err := getRegistry(ctx, client, int64(registryID))
	if err != nil {
		if isAPINotFound(err) {
			return nil, ErrRegistryNotFound
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	eidStr := strconv.Itoa(endpointID)
	policies, ok := registry.RegistryAccesses[eidStr]
	if !ok {
		return &registryAccessPolicies{
			UserAccessPolicies: make(map[string]accessPolicy),
			TeamAccessPolicies: make(map[string]accessPolicy),
		}, nil
	}

	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]accessPolicy)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]accessPolicy)
	}

	return &policies, nil
}

func resourceRegistryAccessCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")
	roleID := int64(d.Get("role_id").(int))

	if !hasTeam && !hasUser {
		return diag.FromErr(fmt.Errorf("either team_id or user_id must be provided"))
	}

	policies, err := getRegistryPolicies(ctx, client, registryID, endpointID)
	if err != nil {
		return diag.FromErr(err)
	}

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		policies.TeamAccessPolicies[tidStr] = accessPolicy{RoleID: roleID}
	}
	if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		policies.UserAccessPolicies[uidStr] = accessPolicy{RoleID: roleID}
	}

	if err := putRegistryAccess(ctx, client, registryID, endpointID, policies); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update registry access: %w", err))
	}

	id := fmt.Sprintf("%d/%d/", registryID, endpointID)
	if hasTeam {
		id += fmt.Sprintf("team/%d", teamID.(int))
	} else {
		id += fmt.Sprintf("user/%d", userID.(int))
	}
	d.SetId(id)

	return resourceRegistryAccessRead(ctx, d, meta)
}

func resourceRegistryAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getRegistryPolicies(ctx, client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	found := false
	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			if err := d.Set("role_id", int(p.RoleID)); err != nil {
				return diag.FromErr(err)
			}
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			if err := d.Set("role_id", int(p.RoleID)); err != nil {
				return diag.FromErr(err)
			}
			found = true
		}
	}

	if !found {
		d.SetId("")
	}

	return nil
}

func resourceRegistryAccessUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceRegistryAccessCreate(ctx, d, meta)
}

func resourceRegistryAccessDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getRegistryPolicies(ctx, client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return nil
		}
		return diag.FromErr(err)
	}

	if hasTeam {
		delete(policies.TeamAccessPolicies, strconv.Itoa(teamID.(int)))
	}
	if hasUser {
		delete(policies.UserAccessPolicies, strconv.Itoa(userID.(int)))
	}

	if err := putRegistryAccess(ctx, client, registryID, endpointID, policies); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete registry access: %w", err))
	}

	return nil
}

// putRegistryAccess replaces the registry's access on the environment.
func putRegistryAccess(ctx context.Context, client *APIClient, registryID, endpointID int, policies *registryAccessPolicies) error {
	body := registryAccessPayload{
		UserAccessPolicies: policies.UserAccessPolicies,
		TeamAccessPolicies: policies.TeamAccessPolicies,
		Namespaces:         policies.Namespaces,
	}
	return doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoints/%d/registries/%d", client.Endpoint, endpointID, registryID), body, nil)
}
