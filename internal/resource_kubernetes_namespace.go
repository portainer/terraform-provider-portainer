package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesNamespace() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesNamespaceCreate,
		ReadContext:   resourceKubernetesNamespaceRead,
		UpdateContext: resourceKubernetesNamespaceUpdate,
		DeleteContext: resourceKubernetesNamespaceDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment where the namespace is created.",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Kubernetes namespace (metadata.name).",
			},
			"owner": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Owner label assigned to the namespace in Portainer.",
			},
			"annotations": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Map of annotations applied to the Kubernetes namespace.",
			},
			"resource_quota": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Resource quota limits applied to the namespace (e.g. cpu, memory limits and requests).",
			},
		},
	}
}

func resourceKubernetesNamespaceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	licensed, err := hasLicense(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	annotations := map[string]string{}
	if raw, ok := d.GetOk("annotations"); ok {
		for k, v := range raw.(map[string]interface{}) {
			annotations[k] = v.(string)
		}
	}

	quota := map[string]string{}
	if raw, ok := d.GetOk("resource_quota"); ok {
		for k, v := range raw.(map[string]interface{}) {
			quota[k] = v.(string)
		}
	}

	var rq map[string]interface{}
	if licensed {
		rq = map[string]interface{}{
			"enabled":       true,
			"cpuRequest":    quota["cpu_request"],
			"cpuLimit":      quota["cpu_limit"],
			"memoryRequest": quota["memory_request"],
			"memoryLimit":   quota["memory_limit"],
		}
	} else {
		rq = map[string]interface{}{
			"enabled": true,
			"cpu":     quota["cpu"],
			"memory":  quota["memory"],
		}
	}

	body := map[string]interface{}{
		"Name":          d.Get("name").(string),
		"Owner":         d.Get("owner").(string),
		"Annotations":   annotations,
		"ResourceQuota": rq,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, id)
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
		return diag.FromErr(fmt.Errorf("failed to create namespace: %s", string(data)))
	}

	envID := strconv.Itoa(id)
	d.SetId(fmt.Sprintf("%s:%s", envID, d.Get("name").(string)))
	return resourceKubernetesNamespaceRead(ctx, d, meta)
}

func resourceKubernetesNamespaceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, err := strconv.Atoi(idParts[0])
	if err != nil {
		return diag.FromErr(fmt.Errorf("invalid environment ID in resource ID %q: %w", d.Id(), err))
	}
	name := idParts[1]

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s?withResourceQuota=true", client.Endpoint, envID, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	// Namespace deleted out-of-band — clear from state so the next plan recreates.
	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read namespace %q: %s", name, string(data)))
	}

	var ns struct {
		Name           string            `json:"Name"`
		NamespaceOwner string            `json:"NamespaceOwner"`
		Annotations    map[string]string `json:"Annotations"`
		ResourceQuota  *struct {
			Spec struct {
				Hard map[string]string `json:"hard"`
			} `json:"spec"`
		} `json:"ResourceQuota"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ns); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode namespace response: %w", err))
	}

	if err := d.Set("environment_id", envID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", ns.Name); err != nil {
		return diag.FromErr(err)
	}
	// NamespaceOwner is a Portainer-level label the API does not reliably persist back —
	// GET returns "" even when the namespace was created with an owner. Only overwrite
	// state when the API returns something, so we don't generate permanent drift.
	if ns.NamespaceOwner != "" {
		if err := d.Set("owner", ns.NamespaceOwner); err != nil {
			return diag.FromErr(err)
		}
	}

	if ns.Annotations != nil {
		if err := d.Set("annotations", ns.Annotations); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := d.Set("annotations", map[string]string{}); err != nil {
			return diag.FromErr(err)
		}
	}

	// Map the K8s ResourceQuota spec.hard keys ("limits.cpu", "requests.memory", …) back
	// to the friendlier schema keys (cpu_limit, memory_limit, cpu_request, memory_request).
	// When EnableResourceOverCommit is true Portainer does not persist quotas and the API
	// returns an empty spec.hard; skip d.Set in that case so the authored quota stays in
	// state and does not generate permanent drift on over-commit environments.
	quota := map[string]string{}
	if ns.ResourceQuota != nil {
		hard := ns.ResourceQuota.Spec.Hard
		if v, ok := hard["limits.cpu"]; ok {
			quota["cpu_limit"] = v
		}
		if v, ok := hard["limits.memory"]; ok {
			quota["memory_limit"] = v
		}
		if v, ok := hard["requests.cpu"]; ok {
			quota["cpu_request"] = v
		}
		if v, ok := hard["requests.memory"]; ok {
			quota["memory_request"] = v
		}
	}
	if len(quota) > 0 {
		if err := d.Set("resource_quota", quota); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceKubernetesNamespaceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	licensed, err := hasLicense(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, _ := strconv.Atoi(idParts[0])
	oldName := idParts[1]
	newName := d.Get("name").(string)

	annotations := map[string]string{}
	if raw, ok := d.GetOk("annotations"); ok {
		for k, v := range raw.(map[string]interface{}) {
			annotations[k] = v.(string)
		}
	}

	quota := map[string]string{}
	if raw, ok := d.GetOk("resource_quota"); ok {
		for k, v := range raw.(map[string]interface{}) {
			quota[k] = v.(string)
		}
	}

	var rq map[string]interface{}
	if licensed {
		rq = map[string]interface{}{
			"enabled":       true,
			"cpuRequest":    quota["cpu_request"],
			"cpuLimit":      quota["cpu_limit"],
			"memoryRequest": quota["memory_request"],
			"memoryLimit":   quota["memory_limit"],
		}
	} else {
		rq = map[string]interface{}{
			"enabled": true,
			"cpu":     quota["cpu"],
			"memory":  quota["memory"],
		}
	}

	body := map[string]interface{}{
		"Name":          newName,
		"Owner":         d.Get("owner").(string),
		"Annotations":   annotations,
		"ResourceQuota": rq,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s", client.Endpoint, envID, oldName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
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
		return diag.FromErr(fmt.Errorf("failed to update namespace: %s", string(data)))
	}

	// If name changed, update ID
	if oldName != newName {
		d.SetId(fmt.Sprintf("%d:%s", envID, newName))
	}

	return resourceKubernetesNamespaceRead(ctx, d, meta)
}

func resourceKubernetesNamespaceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id()))
	}
	envID, _ := strconv.Atoi(idParts[0])
	name := idParts[1]

	body := map[string]string{
		"Name": name,
	}
	jsonBody, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, envID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, bytes.NewBuffer(jsonBody))
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
		return diag.FromErr(fmt.Errorf("failed to delete namespace: %s", string(data)))
	}

	d.SetId("")
	return nil
}

func hasLicense(ctx context.Context, client *APIClient) (bool, error) {
	url := fmt.Sprintf("%s/licenses", client.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return false, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var licenses []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&licenses); err != nil {
		return false, err
	}

	return len(licenses) > 0, nil
}
