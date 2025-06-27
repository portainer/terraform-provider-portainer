package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEdgeJob() *schema.Resource {
	return &schema.Resource{
		Create: resourceEdgeJobCreate,
		Read:   resourceEdgeJobRead,
		Update: resourceEdgeJobUpdate,
		Delete: resourceEdgeJobDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"cron_expression": {
				Type:     schema.TypeString,
				Required: true,
			},
			"edge_groups": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeInt},
				Required: true,
			},
			"endpoints": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeInt},
				Required: true,
			},
			"recurring": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"file_content": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"file_content", "file_path"},
			},
			"file_path": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"file_content", "file_path"},
			},
		},
	}
}

func findExistingEdgeJobByName(client *APIClient, name string) (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_jobs", client.Endpoint), nil)
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
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list edge jobs: %s", string(data))
	}

	var jobs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return 0, err
	}

	for _, job := range jobs {
		if job["Name"] == name {
			if id, ok := job["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}

	return 0, nil
}

func resourceEdgeJobCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	name := d.Get("name").(string)

	if existingID, err := findExistingEdgeJobByName(client, name); err != nil {
		return fmt.Errorf("failed to check for existing edge job: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEdgeJobUpdate(d, meta)
	}

	cron := d.Get("cron_expression").(string)
	edgeGroups := d.Get("edge_groups").([]interface{})
	endpoints := d.Get("endpoints").([]interface{})
	recurring := d.Get("recurring").(bool)

	edgeGroupJSON, _ := json.Marshal(edgeGroups)
	endpointJSON, _ := json.Marshal(endpoints)

	if v, ok := d.GetOk("file_content"); ok {
		body := map[string]interface{}{
			"name":           name,
			"cronExpression": cron,
			"edgeGroups":     edgeGroups,
			"endpoints":      endpoints,
			"recurring":      recurring,
			"fileContent":    v.(string),
		}

		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/edge_jobs/create/string", client.Endpoint), bytes.NewBuffer(jsonBody))
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

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to create edge job: %s", string(data))
		}

		var result struct {
			Id int `json:"Id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}
		d.SetId(strconv.Itoa(result.Id))
		return nil
	} else if v, ok := d.GetOk("file_path"); ok {
		path := v.(string)
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("cannot open file: %w", err)
		}
		defer file.Close()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("Name", name)
		_ = writer.WriteField("CronExpression", cron)
		_ = writer.WriteField("EdgeGroups", string(edgeGroupJSON))
		_ = writer.WriteField("Endpoints", string(endpointJSON))
		_ = writer.WriteField("Recurring", strconv.FormatBool(recurring))

		part, err := writer.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			return err
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return err
		}
		writer.Close()

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/edge_jobs/create/file", client.Endpoint), &body)
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

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to create edge job from file: %s", string(data))
		}

		var result struct {
			Id int `json:"Id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return err
		}
		d.SetId(strconv.Itoa(result.Id))
		return nil
	}

	return errors.New("either file_content or file_path must be provided")
}

func resourceEdgeJobRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	jobID := d.Id()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, jobID), nil)
	if err != nil {
		return fmt.Errorf("failed to build edge job read request: %w", err)
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
		return fmt.Errorf("failed to send edge job read request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read edge job: %s", string(data))
	}

	var result struct {
		Name           string                 `json:"Name"`
		CronExpression string                 `json:"CronExpression"`
		EdgeGroups     []int                  `json:"EdgeGroups"`
		Endpoints      map[string]interface{} `json:"Endpoints"` // not mapped back
		Recurring      bool                   `json:"Recurring"`
		ScriptPath     string                 `json:"ScriptPath"` // not mapped back
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode edge job response: %w", err)
	}

	d.Set("name", result.Name)
	d.Set("cron_expression", result.CronExpression)
	d.Set("edge_groups", result.EdgeGroups)
	d.Set("recurring", result.Recurring)

	endpointIDs := make([]int, 0, len(result.Endpoints))
	for k := range result.Endpoints {
		if id, err := strconv.Atoi(k); err == nil {
			endpointIDs = append(endpointIDs, id)
		}
	}
	d.Set("endpoints", endpointIDs)

	return nil
}

func resourceEdgeJobUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":           d.Get("name").(string),
		"cronExpression": d.Get("cron_expression").(string),
		"edgeGroups":     d.Get("edge_groups").([]interface{}),
		"endpoints":      d.Get("endpoints").([]interface{}),
		"recurring":      d.Get("recurring").(bool),
	}

	if v, ok := d.GetOk("file_content"); ok {
		payload["fileContent"] = v.(string)
	}

	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update edge job: %s", string(data))
	}

	return nil
}

func resourceEdgeJobDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, d.Id()), nil)
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
		return fmt.Errorf("failed to delete edge job")
	}

	return nil
}
