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

type EdgeConfiguration struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Type         int    `json:"type"`
	Category     string `json:"category"`
	BaseDir      string `json:"baseDir"`
	EdgeGroupIDs []int  `json:"edgeGroupIDs"`
	Created      int64  `json:"created"`
	CreatedBy    int    `json:"createdBy"`
	Updated      int64  `json:"updated"`
	UpdatedBy    int    `json:"updatedBy"`
	Prev         string `json:"prev"`
}

func resourcePortainerEdgeConfigurations() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerEdgeConfigurationsCreate,
		Read:   resourcePortainerEdgeConfigurationsRead,
		Update: schema.Noop,
		Delete: resourcePortainerEdgeConfigurationsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true, ForceNew: true},
			"type":           {Type: schema.TypeString, Required: true},
			"category":       {Type: schema.TypeString, Optional: true, Default: "", ForceNew: true},
			"base_dir":       {Type: schema.TypeString, Optional: true, Default: ""},
			"edge_group_ids": {Type: schema.TypeList, Required: true, Elem: &schema.Schema{Type: schema.TypeInt}},
			"file_path":      {Type: schema.TypeString, Required: true},
		},
	}
}

func convertToIntSlice(input []interface{}) []int {
	result := make([]int, len(input))
	for i, v := range input {
		result[i] = v.(int)
	}
	return result
}

func resourcePortainerEdgeConfigurationsCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	filePath := d.Get("file_path").(string)
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	payload := map[string]interface{}{
		"name":         d.Get("name").(string),
		"type":         d.Get("type").(string),
		"category":     d.Get("category").(string),
		"baseDir":      d.Get("base_dir").(string),
		"edgeGroupIDs": convertToIntSlice(d.Get("edge_group_ids").([]interface{})),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal edgeConfiguration payload: %w", err)
	}
	_ = writer.WriteField("edgeConfiguration", string(payloadBytes))

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/edge_configurations", client.Endpoint), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create edge configuration: %s", string(respBody))
	}

	filterBytes, _ := json.Marshal(payload)
	getReq, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_configurations", client.Endpoint), bytes.NewReader(filterBytes))
	if err != nil {
		return fmt.Errorf("failed to build GET request for lookup: %w", err)
	}
	getReq.Header.Set("Content-Type", "application/json")
	if client.APIKey != "" {
		getReq.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		getReq.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
	getResp, err := client.HTTPClient.Do(getReq)
	if err != nil {
		return fmt.Errorf("failed to send GET lookup request: %w", err)
	}
	defer getResp.Body.Close()

	var configs []EdgeConfiguration
	if err := json.NewDecoder(getResp.Body).Decode(&configs); err != nil {
		return fmt.Errorf("failed to decode GET response: %w", err)
	}
	if len(configs) == 0 {
		return fmt.Errorf("no edge configuration found after creation")
	}

	d.SetId(strconv.Itoa(configs[0].ID))
	return nil
}

func resourcePortainerEdgeConfigurationsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	id := d.Id()
	rawID := filepath.Base(id)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID), nil)
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
	res, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to read edge configuration: %s", string(body))
	}

	var config EdgeConfiguration
	if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
		return err
	}

	d.Set("name", config.Name)
	d.Set("category", config.Category)
	d.Set("base_dir", config.BaseDir)
	d.Set("edge_group_ids", config.EdgeGroupIDs)
	d.Set("type", strconv.Itoa(config.Type))

	return nil
}

func resourcePortainerEdgeConfigurationsUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	rawID := filepath.Base(d.Id())
	filePath := d.Get("file_path").(string)
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("type", d.Get("type").(string))
	for _, id := range d.Get("edge_group_ids").([]interface{}) {
		_ = writer.WriteField("edgeGroupIDs", strconv.Itoa(id.(int)))
	}

	part, err := writer.CreateFormFile("File", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID), body)
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
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update edge configuration: %s", string(responseBody))
	}

	return nil
}

func resourcePortainerEdgeConfigurationsDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	rawID := filepath.Base(d.Id())
	url := fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID)

	req, err := http.NewRequest("DELETE", url, nil)
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

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete edge configuration: %s", string(body))
	}

	d.SetId("")
	return nil
}
