package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceHelmChart() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceHelmChartRead,

		Schema: map[string]*schema.Schema{
			"repo": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Helm repository to read from.",
			},
			"chart": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Chart to inspect. Leave unset to fetch the repository index instead.",
			},
			"command": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "values",
				ValidateFunc: validation.StringInSlice([]string{"values", "readme", "chart"}, false),
				Description:  "What to fetch for `chart`: its default `values`, its `readme`, or the `chart` metadata. Ignored when `chart` is unset.",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Chart version to inspect. Leave unset for the newest one.",
			},
			// Computed attributes
			"content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The requested document, as returned by Portainer: the repository index when `chart` is unset, otherwise the chart's values, readme or metadata.",
			},
		},
	}
}

func dataSourceHelmChartRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	repo := d.Get("repo").(string)
	chart := d.Get("chart").(string)

	q := url.Values{}
	q.Set("repo", repo)

	var target, id string
	if chart == "" {
		target = client.Endpoint + "/templates/helm?" + q.Encode()
		id = "helm-index-" + repo
	} else {
		command := d.Get("command").(string)
		q.Set("chart", chart)
		if v, ok := d.GetOk("version"); ok {
			q.Set("version", v.(string))
		}
		target = fmt.Sprintf("%s/templates/helm/%s?%s", client.Endpoint, command, q.Encode())
		id = fmt.Sprintf("helm-%s-%s-%s", command, repo, chart)
	}

	// Both endpoints answer with a document (YAML or Markdown), not JSON, so
	// this goes through the raw byte helper.
	body, status, err := apiGETWithCodeCtx(ctx, target, client.APIKey, client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read from the Helm repository %s: %w", repo, err))
	}
	if status >= http.StatusBadRequest {
		return diag.FromErr(fmt.Errorf("failed to read from the Helm repository %s: status %d: %s", repo, status, strings.TrimSpace(string(body))))
	}

	if err := d.Set("content", string(body)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id)
	return nil
}
