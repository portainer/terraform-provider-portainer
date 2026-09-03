package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesDescribe() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesDescribeRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"kind": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Kind of the resource to describe, as `kubectl describe` takes it (for example `pod`, `deployment` or `node`).",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the resource to describe.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace of the resource. Leave unset for a cluster-scoped kind such as `node`.",
			},
			// Computed attributes
			"describe": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Output of `kubectl describe` for the resource, as plain text.",
			},
		},
	}
}

func dataSourceKubernetesDescribeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	q := url.Values{}
	q.Set("kind", d.Get("kind").(string))
	q.Set("name", d.Get("name").(string))
	if v, ok := d.GetOk("namespace"); ok {
		q.Set("namespace", v.(string))
	}

	var result struct {
		Describe string `json:"describe"`
	}
	target := fmt.Sprintf("%s/kubernetes/%d/describe?%s", client.Endpoint, envID, q.Encode())
	if err := doJSON(ctx, client, http.MethodGet, target, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to describe %s/%s: %w", d.Get("kind").(string), d.Get("name").(string), err))
	}

	if err := d.Set("describe", result.Describe); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/%s/%s", envID, d.Get("namespace").(string), d.Get("kind").(string), d.Get("name").(string)))
	return nil
}
