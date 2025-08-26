package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespace() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesNamespaceCreate,
		Read:   resourceKubernetesNamespaceRead,
		Update: resourceKubernetesNamespaceUpdate,
		Delete: resourceKubernetesNamespaceDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"owner": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"annotations": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"resource_quota": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceKubernetesNamespaceCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	licensed, err := hasLicense(client)
	if err != nil {
		return err
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
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to create namespace: %s", string(data))
	}

	envID := strconv.Itoa(id)
	d.SetId(fmt.Sprintf("%s:%s", envID, d.Get("name").(string)))
	return resourceKubernetesNamespaceRead(d, meta)
}

func resourceKubernetesNamespaceRead(d *schema.ResourceData, meta interface{}) error {
	// No-op for now
	return nil
}

func resourceKubernetesNamespaceUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	licensed, err := hasLicense(client)
	if err != nil {
		return err
	}

	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id())
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
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to update namespace: %s", string(data))
	}

	// If name changed, update ID
	if oldName != newName {
		d.SetId(fmt.Sprintf("%d:%s", envID, newName))
	}

	return resourceKubernetesNamespaceRead(d, meta)
}

func resourceKubernetesNamespaceDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 2)
	if len(idParts) != 2 {
		return fmt.Errorf("invalid ID format, expected 'envID:name': %s", d.Id())
	}
	envID, _ := strconv.Atoi(idParts[0])
	name := idParts[1]

	body := map[string]string{
		"Name": name,
	}
	jsonBody, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, envID)
	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to delete namespace: %s", string(data))
	}

	d.SetId("")
	return nil
}

func hasLicense(client *APIClient) (bool, error) {
	url := fmt.Sprintf("%s/licenses", client.Endpoint)
	req, err := http.NewRequest("GET", url, nil)
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

	if resp.StatusCode != 200 {
		return false, nil
	}

	var licenses []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&licenses); err != nil {
		return false, err
	}

	return len(licenses) > 0, nil
}
