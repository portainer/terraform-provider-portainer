package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAppTemplates() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAppTemplatesRead,

		Schema: map[string]*schema.Schema{
			"template_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of a template whose stack file should also be fetched. Leave unset to only list the catalogue.",
			},
			// Computed attributes
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version of the template catalogue Portainer is serving.",
			},
			"templates": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Templates in the catalogue Portainer is configured to serve.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":             {Type: schema.TypeInt, Computed: true, Description: "Identifier of the template."},
						"title":          {Type: schema.TypeString, Computed: true, Description: "Title shown in the UI."},
						"description":    {Type: schema.TypeString, Computed: true, Description: "Short description of the template."},
						"type":           {Type: schema.TypeInt, Computed: true, Description: "Template type: 1 = container, 2 = Swarm stack, 3 = Compose stack."},
						"platform":       {Type: schema.TypeString, Computed: true, Description: "Platform the template targets, such as `linux`."},
						"categories":     {Type: schema.TypeList, Computed: true, Elem: &schema.Schema{Type: schema.TypeString}, Description: "Categories the template is filed under."},
						"image":          {Type: schema.TypeString, Computed: true, Description: "Container image the template deploys, empty for a stack template."},
						"repository_url": {Type: schema.TypeString, Computed: true, Description: "Git repository a stack template is built from."},
					},
				},
			},
			"file_content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Stack file of the template named by `template_id`, empty when that argument is unset.",
			},
		},
	}
}

func dataSourceAppTemplatesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var list struct {
		Version   string `json:"version"`
		Templates []struct {
			ID          int      `json:"Id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Type        int      `json:"type"`
			Platform    string   `json:"platform"`
			Categories  []string `json:"categories"`
			Image       string   `json:"image"`
			Repository  struct {
				URL string `json:"url"`
			} `json:"repository"`
		} `json:"templates"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/templates", nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the application templates: %w", err))
	}

	templates := make([]map[string]interface{}, len(list.Templates))
	for i, t := range list.Templates {
		categories := t.Categories
		if categories == nil {
			categories = []string{}
		}
		templates[i] = map[string]interface{}{
			"id": t.ID, "title": t.Title, "description": t.Description,
			"type": t.Type, "platform": t.Platform, "categories": categories,
			"image": t.Image, "repository_url": t.Repository.URL,
		}
	}

	// The file endpoint is a POST even though it only reads — that is how
	// Portainer models it.
	fileContent := ""
	if v, ok := d.GetOk("template_id"); ok {
		var file struct {
			FileContent string `json:"FileContent"`
		}
		url := fmt.Sprintf("%s/templates/%d/file", client.Endpoint, v.(int))
		if err := doJSON(ctx, client, http.MethodPost, url, nil, &file); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read the file of template %d: %w", v.(int), err))
		}
		fileContent = file.FileContent
	}

	if err := setFields(d, map[string]interface{}{
		"version": list.Version, "templates": templates, "file_content": fileContent,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("app-templates-" + list.Version)
	return nil
}
