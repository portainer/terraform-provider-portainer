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

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesHelm() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesHelmCreate,
		Read:   resourceKubernetesHelmRead,
		Delete: resourceKubernetesHelmDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"chart": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"repo": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"values": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
				ForceNew: true,
			},
		},
	}
}

func resourceKubernetesHelmCreate(d *schema.ResourceData, meta interface{}) error {
	timeout := d.Timeout(schema.TimeoutCreate)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to install helm chart: %s", string(data))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", id, d.Get("namespace").(string), d.Get("name").(string)))
	return resourceKubernetesHelmRead(d, meta)
}

func resourceKubernetesHelmRead(d *schema.ResourceData, meta interface{}) error {
	// No-op for now
	return nil
}

func resourceKubernetesHelmDelete(d *schema.ResourceData, meta interface{}) error {
	timeout := d.Timeout(schema.TimeoutDelete)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 3)
	if len(idParts) != 3 {
		return fmt.Errorf("invalid ID format, expected 'envID:namespace:release': %s", d.Id())
	}

	envID := idParts[0]
	namespace := idParts[1]
	release := idParts[2]

	url := fmt.Sprintf("%s/endpoints/%s/kubernetes/helm/%s?namespace=%s", client.Endpoint, envID, release, namespace)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete helm release: %s", string(data))
	}

	d.SetId("")
	return nil
}
