package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePortainerPolicy() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePortainerPolicyRead,

		Schema: map[string]*schema.Schema{
			"policy_id": {
				Type:         schema.TypeInt,
				Optional:     true,
				Description:  "ID of the policy. If set, the policy is looked up by ID directly.",
				ExactlyOneOf: []string{"policy_id", "name"},
			},
			"name": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Name of the policy. If set, the policy is looked up by name from the list.",
				ExactlyOneOf: []string{"policy_id", "name"},
			},
			"environment_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Environment type for the policy.",
			},
			"policy_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Policy type.",
			},
			"environment_groups": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of environment group IDs.",
			},
			"data": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Policy data as a JSON string.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the policy was created.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the policy was last updated.",
			},
		},
	}
}

func dataSourcePortainerPolicyRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// If policy_id is provided, look up directly
	if v, ok := d.GetOk("policy_id"); ok {
		policyID := v.(int)
		return readPolicyByID(d, client, policyID)
	}

	// Otherwise, look up by name from the list
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/policies", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list policies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list policies, status %d: %s", resp.StatusCode, string(data))
	}

	var listResp struct {
		Policies []map[string]interface{} `json:"policies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("failed to decode policy list: %w", err)
	}

	for _, p := range listResp.Policies {
		if pName, ok := p["Name"].(string); ok && pName == name {
			if id, ok := p["Id"].(float64); ok {
				return readPolicyByID(d, client, int(id))
			}
		}
	}

	return fmt.Errorf("policy with name %q not found", name)
}

func readPolicyByID(d *schema.ResourceData, client *APIClient, policyID int) error {
	idStr := strconv.Itoa(policyID)

	resp, err := client.DoRequest("GET", fmt.Sprintf("/policies/%s", idStr), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read policy %d: %w", policyID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read policy %d, status %d: %s", policyID, resp.StatusCode, string(data))
	}

	var policy map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return fmt.Errorf("failed to decode policy response: %w", err)
	}

	d.SetId(idStr)

	if v, ok := policy["Name"]; ok {
		_ = d.Set("name", v)
	}
	if v, ok := policy["EnvironmentType"]; ok {
		_ = d.Set("environment_type", v)
	}
	if v, ok := policy["Type"]; ok {
		_ = d.Set("policy_type", v)
	}
	if v, ok := policy["CreatedAt"]; ok {
		_ = d.Set("created_at", v)
	}
	if v, ok := policy["UpdatedAt"]; ok {
		_ = d.Set("updated_at", v)
	}

	if groups, ok := policy["EnvironmentGroups"]; ok && groups != nil {
		if groupsList, ok := groups.([]interface{}); ok {
			intGroups := make([]int, 0, len(groupsList))
			for _, g := range groupsList {
				if gf, ok := g.(float64); ok {
					intGroups = append(intGroups, int(gf))
				}
			}
			_ = d.Set("environment_groups", intGroups)
		}
	}

	if data, ok := policy["Data"]; ok && data != nil {
		dataJSON, err := json.Marshal(data)
		if err == nil {
			_ = d.Set("data", string(dataJSON))
		}
	}

	return nil
}
