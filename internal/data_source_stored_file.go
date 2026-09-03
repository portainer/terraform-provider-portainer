package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Portainer stores the definition of a custom template, an edge stack and an
// edge job as a file, each behind its own endpoint returning a single JSON
// string field under a different name. readStoredFile is the shared part; the
// three schemas below are written out rather than generated so docsdriftlint
// can read them statically and keep checking those pages for drift.
func readStoredFile(ctx context.Context, meta interface{}, what, path, jsonField string, id int, d *schema.ResourceData) diag.Diagnostics {
	client := meta.(*APIClient)

	var body map[string]string
	url := fmt.Sprintf("%s"+path, client.Endpoint, id)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &body); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the file of %s %d: %w", what, id, err))
	}

	if err := d.Set("file_content", body[jsonField]); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%d/file", id))
	return nil
}

func dataSourceCustomTemplateFile() *schema.Resource {
	return &schema.Resource{
		ReadContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return readStoredFile(ctx, meta, "custom template", "/custom_templates/%d/file", "FileContent", d.Get("template_id").(int), d)
		},
		Schema: map[string]*schema.Schema{
			"template_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the custom template whose stack file is read.",
			},
			"file_content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Content of the stored file, exactly as Portainer holds it.",
			},
		},
	}
}

func dataSourceEdgeStackFile() *schema.Resource {
	return &schema.Resource{
		ReadContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return readStoredFile(ctx, meta, "edge stack", "/edge_stacks/%d/file", "StackFileContent", d.Get("edge_stack_id").(int), d)
		},
		Schema: map[string]*schema.Schema{
			"edge_stack_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge stack whose stack file is read.",
			},
			"file_content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Content of the stored file, exactly as Portainer holds it.",
			},
		},
	}
}

func dataSourceEdgeJobFile() *schema.Resource {
	return &schema.Resource{
		ReadContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
			return readStoredFile(ctx, meta, "edge job", "/edge_jobs/%d/file", "FileContent", d.Get("edge_job_id").(int), d)
		},
		Schema: map[string]*schema.Schema{
			"edge_job_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the edge job whose script is read.",
			},
			"file_content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Content of the stored file, exactly as Portainer holds it.",
			},
		},
	}
}
