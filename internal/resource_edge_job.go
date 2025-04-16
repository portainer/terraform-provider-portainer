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

func resourceEdgeJobCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	name := d.Get("name").(string)
	cron := d.Get("cron_expression").(string)
	edgeGroups := d.Get("edge_groups").([]interface{})
	endpoints := d.Get("endpoints").([]interface{})
	recurring := d.Get("recurring").(bool)

	edgeGroupJSON, _ := json.Marshal(edgeGroups)
	endpointJSON, _ := json.Marshal(endpoints)

	if v, ok := d.GetOk("file_content"); ok {
		// string-based submission
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
		req.Header.Set("X-API-Key", client.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
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
		// file-based submission
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
		req.Header.Set("X-API-Key", client.APIKey)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := http.DefaultClient.Do(req)
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
	// Optional: implement if needed
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
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete edge job")
	}

	return nil
}
