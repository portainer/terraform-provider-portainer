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
	ID           int         `json:"id"`
	Name         string      `json:"name"`
	Type         int         `json:"type"`
	Category     string      `json:"category"`
	BaseDir      string      `json:"baseDir"`
	EdgeGroupIDs []int       `json:"edgeGroupIDs"`
	Created      int64       `json:"created"`
	CreatedBy    int         `json:"createdBy"`
	Updated      int64       `json:"updated"`
	UpdatedBy    int         `json:"updatedBy"`
	Prev         interface{} `json:"prev"`
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read create response: %w", err)
	}

	var created EdgeConfiguration
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &created); err != nil {
			return fmt.Errorf("failed to decode create response: %w", err)
		}
	}

	if created.ID == 0 {
		// API returned empty body — find the created config by name
		name := d.Get("name").(string)
		listReq, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_configurations", client.Endpoint), nil)
		if err != nil {
			return fmt.Errorf("failed to build list request: %w", err)
		}
		if client.APIKey != "" {
			listReq.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			listReq.Header.Set("Authorization", "Bearer "+client.JWTToken)
		}
		listResp, err := client.HTTPClient.Do(listReq)
		if err != nil {
			return fmt.Errorf("failed to list edge configurations: %w", err)
		}
		defer listResp.Body.Close()
		var configs []EdgeConfiguration
		if err := json.NewDecoder(listResp.Body).Decode(&configs); err != nil {
			return fmt.Errorf("failed to decode edge configurations list: %w", err)
		}
		for _, c := range configs {
			if c.Name == name {
				created = c
				break
			}
		}
		if created.ID == 0 {
			return fmt.Errorf("edge configuration created but could not determine its ID")
		}
	}

	d.SetId(strconv.Itoa(created.ID))
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
