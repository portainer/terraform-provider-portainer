package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesDeleteObject() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesDeleteObjectCreate,
		ReadContext:   resourceKubernetesDeleteObjectRead,
		DeleteContext: resourceKubernetesDeleteObjectDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment from which the resources are deleted.",
			},
			"resource_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"cron_jobs", "ingresses", "jobs", "role_bindings",
					"roles", "service_accounts", "services", "cluster_role_bindings",
					"cluster_roles",
				}, false),
				Description: "Type of Kubernetes resource to delete. One of `cron_jobs`, `ingresses`, `jobs`, `role_bindings`, `roles`, `service_accounts`, `services`, `cluster_role_bindings`, `cluster_roles`.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace containing the resources to delete (ignored for cluster-scoped types).",
			},
			"names": {
				Type:        schema.TypeList,
				Required:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of resource names to delete within the target namespace or cluster scope.",
			},
		},
	}
}

func resourceKubernetesDeleteObjectCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	typePath := d.Get("resource_type").(string)
	namespace := d.Get("namespace").(string)
	nameList := d.Get("names").([]interface{})

	names := make([]string, len(nameList))
	for i, v := range nameList {
		names[i] = v.(string)
	}

	body := map[string][]string{
		namespace: names,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/%s/delete", client.Endpoint, envID, typePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete %s: %s", typePath, string(data)))
	}

	id := fmt.Sprintf("%d:%s:%s", envID, typePath, strings.Join(names, ","))
	d.SetId(id)
	return nil
}

func resourceKubernetesDeleteObjectRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceKubernetesDeleteObjectDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
