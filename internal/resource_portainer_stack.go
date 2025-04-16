package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerStack() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerStackCreate,
		Read:   resourcePortainerStackRead,
		Delete: resourcePortainerStackDelete,
		Update: resourcePortainerStackUpdate,
		Schema: map[string]*schema.Schema{
			"deployment_type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Deployment mode: 'standalone', 'swarm', or 'kubernetes'",
				ForceNew:    true,
			},
			"method": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Creation method: 'string', 'file', 'repository', or 'url'",
				ForceNew:    true,
			},
			"name":                {Type: schema.TypeString, Required: true, ForceNew: true},
			"endpoint_id":         {Type: schema.TypeInt, Required: true, ForceNew: true},
			"swarm_id":            {Type: schema.TypeString, Optional: true, ForceNew: true, Computed: true},
			"namespace":           {Type: schema.TypeString, Optional: true, ForceNew: true},
			"stack_file_content":  {Type: schema.TypeString, Optional: true},
			"stack_file_path":     {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_url":      {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_username": {Type: schema.TypeString, Optional: true},
			"repository_password": {Type: schema.TypeString, Optional: true, Sensitive: true},
			"repository_reference_name": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "refs/heads/main",
			},
			"file_path_in_repository": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "docker-compose.yml",
			},
			"manifest_url":   {Type: schema.TypeString, Optional: true, ForceNew: true},
			"compose_format": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"env": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":  {Type: schema.TypeString, Required: true},
						"value": {Type: schema.TypeString, Required: true},
					},
				},
			},
			"tlsskip_verify": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
		},
	}
}

func resourcePortainerStackCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	deployment := d.Get("deployment_type").(string)
	method := d.Get("method").(string)

	if deployment == "swarm" && d.Get("swarm_id") == "" {
		swarmID, err := fetchSwarmID(client, d.Get("endpoint_id").(int))
		if err != nil {
			return fmt.Errorf("failed to fetch swarm_id: %w", err)
		}
		d.Set("swarm_id", swarmID)
	}

	switch deployment {
	case "standalone":
		switch method {
		case "string":
			return createStackStandaloneString(d, client)
		case "file":
			return createStackStandaloneFile(d, client)
		case "repository":
			return createStackStandaloneRepo(d, client)
		}
	case "swarm":
		switch method {
		case "string":
			return createStackSwarmString(d, client)
		case "file":
			return createStackSwarmFile(d, client)
		case "repository":
			return createStackSwarmRepo(d, client)
		}
	case "kubernetes":
		switch method {
		case "string":
			return createStackK8sString(d, client)
		case "repository":
			return createStackK8sRepo(d, client)
		case "url":
			return createStackK8sURL(d, client)
		}
	}
	return fmt.Errorf("invalid combination of deployment_type and method")
}

func resourcePortainerStackRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func fetchSwarmID(client *APIClient, endpointID int) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch swarm info: %s", string(data))
	}

	var swarm struct {
		ID string `json:"ID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&swarm); err != nil {
		return "", err
	}
	return swarm.ID, nil
}

func resourcePortainerStackDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()
	endpointID := d.Get("endpoint_id").(int)

	url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, id, endpointID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete stack: %s", string(data))
}

func resourcePortainerStackUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	stackID := d.Id()
	endpointID := d.Get("endpoint_id").(int)
	method := d.Get("method").(string)

	if method == "repository" {
		payload := map[string]interface{}{
			"env":                      flattenEnvList(d.Get("env").([]interface{})),
			"prune":                    true,
			"pullImage":                false,
			"repositoryAuthentication": true,
			"repositoryUsername":       d.Get("repository_username").(string),
			"repositoryPassword":       d.Get("repository_password").(string),
			"repositoryReferenceName":  d.Get("repository_reference_name").(string),
			"stackName":                d.Get("name").(string),
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		u := fmt.Sprintf("%s/stacks/%s/git/redeploy?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequest("PUT", u, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", client.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update git stack: %s", string(data))
		}
		return nil
	}

	// fallback to default update (string based)
	payload := map[string]interface{}{
		"env":              flattenEnvList(d.Get("env").([]interface{})),
		"stackFileContent": d.Get("stack_file_content").(string),
		"prune":            true,
		"pullImage":        false,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	u := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, stackID, endpointID)
	req, err := http.NewRequest("PUT", u, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update stack: %s", string(data))
	}

	return nil
}

func flattenEnvList(envList []interface{}) []map[string]string {
	var out []map[string]string
	for _, v := range envList {
		item := v.(map[string]interface{})
		out = append(out, map[string]string{
			"name":  item["name"].(string),
			"value": item["value"].(string),
		})
	}
	return out
}

func mustJSON(data interface{}) []byte {
	out, _ := json.Marshal(data)
	return out
}

// --------------------- STANDALONE ----------------------

func createStackStandaloneString(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"env":              flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":  false,
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackStandaloneFile(d *schema.ResourceData, client *APIClient) error {
	path := d.Get("stack_file_path").(string)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("Name", d.Get("name").(string))
	writer.WriteField("Env", string(mustJSON(flattenEnvList(d.Get("env").([]interface{})))))

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	io.Copy(part, file)
	writer.Close()

	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/file?endpointId=%d", client.Endpoint, endpointID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", client.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack from file: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackStandaloneRepo(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":                     d.Get("name").(string),
		"composeFile":              d.Get("file_path_in_repository").(string),
		"repositoryURL":            d.Get("repository_url").(string),
		"repositoryUsername":       d.Get("repository_username").(string),
		"repositoryPassword":       d.Get("repository_password").(string),
		"repositoryReferenceName":  d.Get("repository_reference_name").(string),
		"repositoryAuthentication": true,
		"env":                      flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":          false,
		"tlsskipVerify":            d.Get("tlsskip_verify").(bool),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

// --------------------- SWARM ----------------------

func createStackSwarmString(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"env":              flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":  false,
		"swarmID":          d.Get("swarm_id").(string),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackSwarmFile(d *schema.ResourceData, client *APIClient) error {
	path := d.Get("stack_file_path").(string)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("Name", d.Get("name").(string))
	writer.WriteField("Env", string(mustJSON(flattenEnvList(d.Get("env").([]interface{})))))
	writer.WriteField("SwarmID", d.Get("swarm_id").(string))

	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return err
	}
	io.Copy(part, file)
	writer.Close()

	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/file?endpointId=%d", client.Endpoint, endpointID)
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", client.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack from file: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackSwarmRepo(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":                     d.Get("name").(string),
		"composeFile":              d.Get("file_path_in_repository").(string),
		"repositoryURL":            d.Get("repository_url").(string),
		"repositoryUsername":       d.Get("repository_username").(string),
		"repositoryPassword":       d.Get("repository_password").(string),
		"repositoryReferenceName":  d.Get("repository_reference_name").(string),
		"repositoryAuthentication": true,
		"env":                      flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":          false,
		"tlsskipVerify":            d.Get("tlsskip_verify").(bool),
		"swarmID":                  d.Get("swarm_id").(string),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

// --------------------- KUBERNETES ----------------------

func createStackK8sString(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":        d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"namespace":        d.Get("namespace").(string),
		"composeFormat":    d.Get("compose_format").(bool),
		"fromAppTemplate":  false,
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from string: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sRepo(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":                d.Get("name").(string),
		"manifestFile":             d.Get("file_path_in_repository").(string),
		"namespace":                d.Get("namespace").(string),
		"composeFormat":            d.Get("compose_format").(bool),
		"repositoryURL":            d.Get("repository_url").(string),
		"repositoryUsername":       d.Get("repository_username").(string),
		"repositoryPassword":       d.Get("repository_password").(string),
		"repositoryReferenceName":  d.Get("repository_reference_name").(string),
		"repositoryAuthentication": true,
		"tlsskipVerify":            d.Get("tlsskip_verify").(bool),
		"fromAppTemplate":          false,
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sURL(d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":     d.Get("name").(string),
		"manifestURL":   d.Get("manifest_url").(string),
		"namespace":     d.Get("namespace").(string),
		"composeFormat": d.Get("compose_format").(bool),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/url?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from URL: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}
