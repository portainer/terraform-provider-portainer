package internal

import (
	"context"
	"encoding/json"
	"fmt"
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
		return diag.FromErr(readPolicyTemplateByID(ctx, d, client, templateID))
	}

	// Otherwise, look up by name from the list
	name := d.Get("name").(string)

	var listResp struct {
		Templates []map[string]interface{} `json:"templates"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/policies/templates", nil, &listResp); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list policy templates: %w", err))
	}

	for _, t := range listResp.Templates {
		if tName, ok := t["name"].(string); ok && tName == name {
			if id, ok := t["id"].(string); ok {
				return diag.FromErr(readPolicyTemplateByID(ctx, d, client, id))
			}
		}
	}

	return diag.FromErr(fmt.Errorf("policy template with name %q not found", name))
}

func readPolicyTemplateByID(ctx context.Context, d *schema.ResourceData, client *APIClient, templateID string) error {
	var tmpl map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/policies/templates/%s", client.Endpoint, templateID), nil, &tmpl); err != nil {
		return fmt.Errorf("failed to read policy template %s: %w", templateID, err)
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
