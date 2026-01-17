package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

type RegistryAccessPolicies struct {
	UserAccessPolicies map[string]map[string]int `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]map[string]int `json:"TeamAccessPolicies"`
	Namespaces         []string                  `json:"Namespaces"`
}

func getRegistryPolicies(client *APIClient, registryID int, endpointID int) (*RegistryAccessPolicies, error) {
	resp, err := client.DoRequest("GET", fmt.Sprintf("/registries/%d", registryID), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrRegistryNotFound
	}

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch registry: %s", string(data))
	}

	var registry struct {
		RegistryAccesses map[string]RegistryAccessPolicies `json:"RegistryAccesses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, err
	}

	eidStr := strconv.Itoa(endpointID)
	policies, ok := registry.RegistryAccesses[eidStr]
	if !ok {
		return &RegistryAccessPolicies{
			UserAccessPolicies: make(map[string]map[string]int),
			TeamAccessPolicies: make(map[string]map[string]int),
		}, nil
	}

	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]map[string]int)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]map[string]int)
	}

	return &policies, nil
}

func resourceRegistryAccessCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")
	roleID := d.Get("role_id").(int)

	if !hasTeam && !hasUser {
		return fmt.Errorf("either team_id or user_id must be provided")
	}

	policies, err := getRegistryPolicies(client, registryID, endpointID)
	if err != nil {
		return err
	}

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		policies.TeamAccessPolicies[tidStr] = map[string]int{"RoleId": roleID}
	}
	if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		policies.UserAccessPolicies[uidStr] = map[string]int{"RoleId": roleID}
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/endpoints/%d/registries/%d", endpointID, registryID), nil, policies)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update registry access: %s", string(data))
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
			d.Set("role_id", p["RoleId"])
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			d.Set("role_id", p["RoleId"])
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

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/endpoints/%d/registries/%d", endpointID, registryID), nil, policies)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete registry access: %s", string(data))
	}

	return nil
}
