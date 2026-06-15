package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesHelm() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesHelmCreate,
		ReadContext:   resourceKubernetesHelmRead,
		DeleteContext: resourceKubernetesHelmDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the Helm chart is installed.",
			},
			"chart": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Helm chart to install.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Helm release name used to install the chart.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace in which the Helm release is created.",
			},
			"repo": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "URL of the Helm chart repository hosting the chart.",
			},
			"values": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				ForceNew:    true,
				Description: "Optional Helm values document (YAML) used to customise the release.",
			},
		},
	}
}

func resourceKubernetesHelmCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	timeout := d.Timeout(schema.TimeoutCreate)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	body := map[string]interface{}{
		"chart":     d.Get("chart").(string),
		"name":      d.Get("name").(string),
		"namespace": d.Get("namespace").(string),
		"repo":      d.Get("repo").(string),
		"values":    d.Get("values").(string),
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/helm", client.Endpoint, id)
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
		return diag.FromErr(fmt.Errorf("failed to install helm chart: %s", string(data)))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", id, d.Get("namespace").(string), d.Get("name").(string)))
	return resourceKubernetesHelmRead(ctx, d, meta)
}

func resourceKubernetesHelmRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// No-op for now
	return nil
}

func resourceKubernetesHelmDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	timeout := d.Timeout(schema.TimeoutDelete)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 3)
	if len(idParts) != 3 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:namespace:release': %s", d.Id()))
	}

	envID := idParts[0]
	namespace := idParts[1]
	release := idParts[2]

	url := fmt.Sprintf("%s/endpoints/%s/kubernetes/helm/%s?namespace=%s", client.Endpoint, envID, release, namespace)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
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

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete helm release: %s", string(data)))
	}

	d.SetId("")
	return nil
}
