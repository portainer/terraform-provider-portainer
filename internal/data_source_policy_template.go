package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePortainerPolicyTemplate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePortainerPolicyTemplateRead,

		Schema: map[string]*schema.Schema{
			"template_id": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "ID of the policy template. If set, the template is looked up by ID directly.",
				ExactlyOneOf: []string{"template_id", "name"},
			},
			"name": {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Name of the policy template. If set, the template is looked up by name from the list.",
				ExactlyOneOf: []string{"template_id", "name"},
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the policy template.",
			},
			"category": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Category of the policy template (rbac, security, setup, registry).",
			},
			"policy_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Policy type of the template.",
			},
			"data": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Template data as a JSON string.",
			},
		},
	}
}

func dataSourcePortainerPolicyTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// If template_id is provided, look up directly
	if v, ok := d.GetOk("template_id"); ok {
		templateID := v.(string)
		return diag.FromErr(readPolicyTemplateByID(d, client, templateID))
	}

	// Otherwise, look up by name from the list
	name := d.Get("name").(string)

	resp, err := client.DoRequest("GET", "/policies/templates", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list policy templates: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list policy templates, status %d: %s", resp.StatusCode, string(data)))
	}

	var listResp struct {
		Templates []map[string]interface{} `json:"templates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode policy template list: %w", err))
	}

	for _, t := range listResp.Templates {
		if tName, ok := t["name"].(string); ok && tName == name {
			if id, ok := t["id"].(string); ok {
				return diag.FromErr(readPolicyTemplateByID(d, client, id))
			}
		}
	}

	return diag.FromErr(fmt.Errorf("policy template with name %q not found", name))
}

func readPolicyTemplateByID(d *schema.ResourceData, client *APIClient, templateID string) error {
	resp, err := client.DoRequest("GET", fmt.Sprintf("/policies/templates/%s", templateID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read policy template %s: %w", templateID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read policy template %s, status %d: %s", templateID, resp.StatusCode, string(data))
	}

	var tmpl map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tmpl); err != nil {
		return fmt.Errorf("failed to decode policy template response: %w", err)
	}

	d.SetId(templateID)

	fields := map[string]interface{}{}
	if v, ok := tmpl["name"]; ok {
		fields["name"] = v
	}
	if v, ok := tmpl["description"]; ok {
		fields["description"] = v
	}
	if v, ok := tmpl["category"]; ok {
		fields["category"] = v
	}
	if v, ok := tmpl["type"]; ok {
		fields["policy_type"] = v
	}

	if data, ok := tmpl["data"]; ok && data != nil {
		dataJSON, err := json.Marshal(data)
		if err == nil {
			fields["data"] = string(dataJSON)
		}
	}

	return setFields(d, fields)
}
