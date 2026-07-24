package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePortainerPolicy() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePortainerPolicyRead,

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

func dataSourcePortainerPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// If policy_id is provided, look up directly
	if v, ok := d.GetOk("policy_id"); ok {
		policyID := v.(int)
		return diag.FromErr(readPolicyByID(ctx, d, client, policyID))
	}

	// Otherwise, look up by name from the list
	name := d.Get("name").(string)

	var listResp struct {
		Policies []map[string]interface{} `json:"policies"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/policies", nil, &listResp); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list policies: %w", err))
	}

	for _, p := range listResp.Policies {
		if pName, ok := p["Name"].(string); ok && pName == name {
			if id, ok := p["Id"].(float64); ok {
				return diag.FromErr(readPolicyByID(ctx, d, client, int(id)))
			}
		}
	}

	return diag.FromErr(fmt.Errorf("policy with name %q not found", name))
}

func readPolicyByID(ctx context.Context, d *schema.ResourceData, client *APIClient, policyID int) error {
	idStr := strconv.Itoa(policyID)

	var policy map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/policies/%s", client.Endpoint, idStr), nil, &policy); err != nil {
		return fmt.Errorf("failed to read policy %d: %w", policyID, err)
	}

	d.SetId(idStr)

	fields := map[string]interface{}{}
	if v, ok := policy["Name"]; ok {
		fields["name"] = v
	}
	if v, ok := policy["EnvironmentType"]; ok {
		fields["environment_type"] = v
	}
	if v, ok := policy["Type"]; ok {
		fields["policy_type"] = v
	}
	if v, ok := policy["CreatedAt"]; ok {
		fields["created_at"] = v
	}
	if v, ok := policy["UpdatedAt"]; ok {
		fields["updated_at"] = v
	}

	if groups, ok := policy["EnvironmentGroups"]; ok && groups != nil {
		if groupsList, ok := groups.([]interface{}); ok {
			intGroups := make([]int, 0, len(groupsList))
			for _, g := range groupsList {
				if gf, ok := g.(float64); ok {
					intGroups = append(intGroups, int(gf))
				}
			}
			fields["environment_groups"] = intGroups
		}
	}

	if data, ok := policy["Data"]; ok && data != nil {
		dataJSON, err := json.Marshal(data)
		if err == nil {
			fields["data"] = string(dataJSON)
		}
	}

	return setFields(d, fields)
}
