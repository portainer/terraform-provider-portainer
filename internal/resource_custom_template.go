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
		},
	}
}

func resourceCustomTemplateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	if v, ok := d.GetOk("file_content"); ok {
		return createTemplateFromString(d, client, v.(string))
	} else if v, ok := d.GetOk("file_path"); ok {
		return createTemplateFromFile(d, client, v.(string))
	} else if v, ok := d.GetOk("repository_url"); ok {
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

func createTemplateFromFile(d *schema.ResourceData, client *APIClient, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	writer.WriteField("Title", d.Get("title").(string))
	writer.WriteField("Description", d.Get("description").(string))
	writer.WriteField("Note", d.Get("note").(string))
	writer.WriteField("Platform", strconv.Itoa(d.Get("platform").(int)))
	writer.WriteField("Type", strconv.Itoa(d.Get("type").(int)))
	writer.WriteField("Logo", d.Get("logo").(string))
	writer.WriteField("EdgeTemplate", strconv.FormatBool(d.Get("edge_template").(bool)))
	writer.WriteField("IsComposeFormat", strconv.FormatBool(d.Get("is_compose_format").(bool)))

	varsJSON, _ := json.Marshal(getVariables(d))
	writer.WriteField("Variables", string(varsJSON))

	part, err := writer.CreateFormFile("File", filepath.Base(path))
	if err != nil {
		return err
	}
	io.Copy(part, file)
	writer.Close()

	req, err := http.NewRequest("POST", client.Endpoint+"/custom_templates/create/file", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create custom template from file: %s", string(data))
	}

	var result struct {
		Id int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.Id))
	return nil
}

func createTemplateFromRepository(d *schema.ResourceData, client *APIClient, repoURL string) error {
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
		"repositoryUsername":          d.Get("repository_username").(string),
		"repositoryPassword":          d.Get("repository_password").(string),
		"repositoryReferenceName":     d.Get("repository_reference").(string),
		"composeFilePathInRepository": d.Get("compose_file_path").(string),
		"tlsskipVerify":               d.Get("tlsskip_verify").(bool),
		"variables":                   getVariables(d),
	}

	return postTemplateJSON(d, client, payload, "/custom_templates/create/repository")
}

func postTemplateJSON(d *schema.ResourceData, client *APIClient, payload map[string]interface{}, endpoint string) error {
	jsonBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", client.Endpoint+endpoint, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
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

	if v, ok := d.GetOk("file_content"); ok {
		payload["fileContent"] = v.(string)
	}
	if v, ok := d.GetOk("repository_url"); ok {
		isGitBased = true
		payload["repositoryURL"] = v.(string)
		payload["repositoryUsername"] = d.Get("repository_username").(string)
		payload["repositoryPassword"] = d.Get("repository_password").(string)
		payload["repositoryReferenceName"] = d.Get("repository_reference").(string)
		payload["repositoryAuthentication"] = true
	}

	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/custom_templates/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update custom template: %s", string(data))
	}

	if isGitBased {
		// Also trigger git_fetch after successful update
		u := fmt.Sprintf("%s/custom_templates/%s/git_fetch", client.Endpoint, d.Id())
		req, err := http.NewRequest("PUT", u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", client.APIKey)

		resp, err := http.DefaultClient.Do(req)
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
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
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
