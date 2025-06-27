package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		Create: resourceCustomTemplateCreate,
		Read:   resourceCustomTemplateRead,
		Delete: resourceCustomTemplateDelete,
		Update: resourceCustomTemplateUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"title":                {Type: schema.TypeString, Required: true},
			"description":          {Type: schema.TypeString, Required: true},
			"note":                 {Type: schema.TypeString, Required: true},
			"platform":             {Type: schema.TypeInt, Required: true},
			"type":                 {Type: schema.TypeInt, Required: true},
			"logo":                 {Type: schema.TypeString, Optional: true},
			"edge_template":        {Type: schema.TypeBool, Optional: true, Default: false},
			"is_compose_format":    {Type: schema.TypeBool, Optional: true, Default: false},
			"variables":            {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeMap}},
			"file_content":         {Type: schema.TypeString, Optional: true},
			"file_path":            {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_url":       {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_username":  {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_password":  {Type: schema.TypeString, Optional: true, Sensitive: true, ForceNew: true},
			"repository_reference": {Type: schema.TypeString, Optional: true, Default: "refs/heads/main", ForceNew: true},
			"compose_file_path":    {Type: schema.TypeString, Optional: true, Default: "docker-compose.yml", ForceNew: true},
			"tlsskip_verify":       {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"repository_authentication": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Enable authentication for the Git repository (default: false).",
			},
		},
	}
}

func findExistingCustomTemplateByTitle(client *APIClient, title string) (int, error) {
	req, err := http.NewRequest("GET", client.Endpoint+"/custom_templates", nil)
	if err != nil {
		return 0, err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list custom templates: %s", string(body))
	}

	var templates []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&templates); err != nil {
		return 0, err
	}

	for _, tmpl := range templates {
		if tmpl["Title"] == title {
			if id, ok := tmpl["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}

	return 0, nil
}

func resourceCustomTemplateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	title := d.Get("title").(string)

	existingID, err := findExistingCustomTemplateByTitle(client, title)
	if err != nil {
		return fmt.Errorf("failed to check for existing custom template: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceCustomTemplateUpdate(d, meta)
	}

	if v, ok := d.GetOk("file_content"); ok {
		return createTemplateFromString(d, client, v.(string))
	}

	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return fmt.Errorf("failed to read template file from path: %w", err)
		}
		d.Set("file_content", string(content))
		return createTemplateFromString(d, client, string(content))
	}

	if v, ok := d.GetOk("repository_url"); ok {
		return createTemplateFromRepository(d, client, v.(string))
	}

	return fmt.Errorf("one of file_content, file_path, or repository_url must be provided")
}

func createTemplateFromString(d *schema.ResourceData, client *APIClient, content string) error {
	payload := map[string]interface{}{
		"title":           d.Get("title").(string),
		"description":     d.Get("description").(string),
		"note":            d.Get("note").(string),
		"platform":        d.Get("platform").(int),
		"type":            d.Get("type").(int),
		"logo":            d.Get("logo").(string),
		"edgeTemplate":    d.Get("edge_template").(bool),
		"isComposeFormat": d.Get("is_compose_format").(bool),
		"fileContent":     content,
		"variables":       getVariables(d),
	}
	return postTemplateJSON(d, client, payload, "/custom_templates/create/string")
}

func createTemplateFromRepository(d *schema.ResourceData, client *APIClient, repoURL string) error {
	useAuth := d.Get("repository_authentication").(bool)
	payload := map[string]interface{}{
		"title":                       d.Get("title").(string),
		"description":                 d.Get("description").(string),
		"note":                        d.Get("note").(string),
		"platform":                    d.Get("platform").(int),
		"type":                        d.Get("type").(int),
		"logo":                        d.Get("logo").(string),
		"edgeTemplate":                d.Get("edge_template").(bool),
		"isComposeFormat":             d.Get("is_compose_format").(bool),
		"repositoryURL":               repoURL,
		"repositoryAuthentication":    useAuth,
		"repositoryReferenceName":     d.Get("repository_reference").(string),
		"composeFilePathInRepository": d.Get("compose_file_path").(string),
		"tlsskipVerify":               d.Get("tlsskip_verify").(bool),
		"variables":                   getVariables(d),
	}

	if useAuth {
		payload["repositoryUsername"] = d.Get("repository_username").(string)
		payload["repositoryPassword"] = d.Get("repository_password").(string)
	}

	return postTemplateJSON(d, client, payload, "/custom_templates/create/repository")
}

func postTemplateJSON(d *schema.ResourceData, client *APIClient, payload map[string]interface{}, endpoint string) error {
	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", client.Endpoint+endpoint, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create custom template: %s", string(data))
	}

	var result struct {
		Id int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.Id))
	return nil
}

func getVariables(d *schema.ResourceData) []interface{} {
	if v, ok := d.GetOk("variables"); ok {
		return v.([]interface{})
	}
	return []interface{}{}
}

func resourceCustomTemplateRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/custom_templates/%s", client.Endpoint, d.Id()), nil)
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

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("failed to read custom template")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("title", result["Title"])
	d.Set("description", result["Description"])
	d.Set("note", result["Note"])
	d.Set("platform", result["Platform"])
	d.Set("type", result["Type"])
	d.Set("logo", result["Logo"])
	d.Set("edge_template", result["EdgeTemplate"])
	d.Set("is_compose_format", result["IsComposeFormat"])

	return nil
}

func resourceCustomTemplateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"title":                       d.Get("title").(string),
		"description":                 d.Get("description").(string),
		"note":                        d.Get("note").(string),
		"platform":                    d.Get("platform").(int),
		"type":                        d.Get("type").(int),
		"logo":                        d.Get("logo").(string),
		"edgeTemplate":                d.Get("edge_template").(bool),
		"isComposeFormat":             d.Get("is_compose_format").(bool),
		"composeFilePathInRepository": d.Get("compose_file_path").(string),
		"tlsskipVerify":               d.Get("tlsskip_verify").(bool),
		"variables":                   getVariables(d),
	}

	isGitBased := false

	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return fmt.Errorf("failed to read template file from path: %w", err)
		}
		d.Set("file_content", string(content))
		payload["fileContent"] = string(content)
	} else if v, ok := d.GetOk("file_content"); ok {
		payload["fileContent"] = v.(string)
	}

	useAuth := d.Get("repository_authentication").(bool)
	if v, ok := d.GetOk("repository_url"); ok {
		isGitBased = true
		payload["repositoryURL"] = v.(string)
		payload["repositoryReferenceName"] = d.Get("repository_reference").(string)
		payload["repositoryAuthentication"] = useAuth
		if useAuth {
			payload["repositoryUsername"] = d.Get("repository_username").(string)
			payload["repositoryPassword"] = d.Get("repository_password").(string)
		}
	}

	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/custom_templates/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update custom template: %s", string(data))
	}

	if isGitBased {
		u := fmt.Sprintf("%s/custom_templates/%s/git_fetch", client.Endpoint, d.Id())
		req, err := http.NewRequest("PUT", u, nil)
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

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to git_fetch template: %s", string(data))
		}
	}

	return resourceCustomTemplateRead(d, meta)
}

func resourceCustomTemplateDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/custom_templates/%s", client.Endpoint, d.Id()), nil)
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

	if resp.StatusCode != 204 && resp.StatusCode != 404 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete custom template: %s", string(data))
	}
	return nil
}
