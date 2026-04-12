package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerPolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerPolicyCreate,
		Read:   resourcePortainerPolicyRead,
		Update: resourcePortainerPolicyUpdate,
		Delete: resourcePortainerPolicyDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the policy.",
			},
			"environment_type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"kubernetes", "docker", "podman", "swarm",
				}, false),
				Description: "Environment type for the policy (kubernetes, docker, podman, swarm).",
			},
			"policy_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"rbac-k8s", "rbac-docker",
					"security-k8s", "security-docker",
					"setup-k8s", "setup-docker",
					"registry-k8s", "registry-docker",
				}, false),
				Description: "Policy type (e.g. rbac-k8s, security-docker).",
			},
			"environment_groups": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of environment group IDs to which the policy applies.",
			},
			"data": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Policy data as a JSON string. Structure depends on policy_type.",
			},
			"allow_override": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to allow environments to override this policy.",
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

func resourcePortainerPolicyCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := buildPolicyPayload(d)

	resp, err := client.DoRequest("POST", "/policies", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create policy, status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode policy create response: %w", err)
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourcePortainerPolicyRead(d, meta)
}

func resourcePortainerPolicyRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", fmt.Sprintf("/policies/%s", d.Id()), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read policy, status %d: %s", resp.StatusCode, string(data))
	}

	var policy map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return fmt.Errorf("failed to decode policy response: %w", err)
	}

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

func resourcePortainerPolicyUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := buildPolicyPayload(d)

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/policies/%s", d.Id()), nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update policy, status %d: %s", resp.StatusCode, string(data))
	}

	return resourcePortainerPolicyRead(d, meta)
}

func resourcePortainerPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/policies/%s", d.Id()), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete policy, status %d: %s", resp.StatusCode, string(data))
	}

	return nil
}

func buildPolicyPayload(d *schema.ResourceData) map[string]interface{} {
	payload := map[string]interface{}{
		"Name":            d.Get("name").(string),
		"EnvironmentType": d.Get("environment_type").(string),
		"Type":            d.Get("policy_type").(string),
		"AllowOverride":   d.Get("allow_override").(bool),
	}

	if v, ok := d.GetOk("environment_groups"); ok {
		groups := v.([]interface{})
		intGroups := make([]int, 0, len(groups))
		for _, g := range groups {
			intGroups = append(intGroups, g.(int))
		}
		payload["EnvironmentGroups"] = intGroups
	}

	if v, ok := d.GetOk("data"); ok {
		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(v.(string)), &dataMap); err == nil {
			payload["Data"] = dataMap
		}
	}

	return payload
}
