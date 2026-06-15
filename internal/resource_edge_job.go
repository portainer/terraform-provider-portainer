package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceEdgeJob() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEdgeJobCreate,
		ReadContext:   resourceEdgeJobRead,
		UpdateContext: resourceEdgeJobUpdate,
		DeleteContext: resourceEdgeJobDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Portainer edge job.",
			},
			"cron_expression": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Cron expression that schedules execution of the edge job.",
			},
			"edge_groups": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Required:    true,
				Description: "List of edge group identifiers targeted by the job.",
			},
			"endpoints": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Required:    true,
				Description: "List of edge endpoint identifiers explicitly targeted by the job (in addition to those in `edge_groups`).",
			},
			"recurring": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the edge job should run repeatedly according to the cron expression (true) or only once (false).",
			},
			"file_content": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"file_content", "file_path"},
				Description:  "Inline script executed by edge agents. Mutually exclusive with `file_path`.",
			},
			"file_path": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ExactlyOneOf: []string{"file_content", "file_path"},
				Description:  "Path on the local filesystem to a script file uploaded as the job payload. Mutually exclusive with `file_content`.",
			},
		},
	}
}

func findExistingEdgeJobByName(ctx context.Context, client *APIClient, name string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_jobs", client.Endpoint), nil)
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

	if resp.StatusCode != http.StatusOK {
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

func resourceEdgeJobCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	name := d.Get("name").(string)

	if existingID, err := findExistingEdgeJobByName(ctx, client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing edge job: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEdgeJobUpdate(ctx, d, meta)
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_jobs/create/string", client.Endpoint), bytes.NewBuffer(jsonBody))
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

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to create edge job: %s", string(data)))
		}

		var result struct {
			Id int `json:"Id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return diag.FromErr(err)
		}
		d.SetId(strconv.Itoa(result.Id))
		return nil
	} else if v, ok := d.GetOk("file_path"); ok {
		path := v.(string)
		file, err := os.Open(path)
		if err != nil {
			return diag.FromErr(fmt.Errorf("cannot open file: %w", err))
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
			return diag.FromErr(err)
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return diag.FromErr(err)
		}
		writer.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_jobs/create/file", client.Endpoint), &body)
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
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to create edge job from file: %s", string(data)))
		}

		var result struct {
			Id int `json:"Id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return diag.FromErr(err)
		}
		d.SetId(strconv.Itoa(result.Id))
		return nil
	}

	return diag.FromErr(errors.New("either file_content or file_path must be provided"))
}

func resourceEdgeJobRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	jobID := d.Id()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, jobID), nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to build edge job read request: %w", err))
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
		return diag.FromErr(fmt.Errorf("failed to send edge job read request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read edge job: %s", string(data)))
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
		return diag.FromErr(fmt.Errorf("failed to decode edge job response: %w", err))
	}

	if err := d.Set("name", result.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("cron_expression", result.CronExpression); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("edge_groups", result.EdgeGroups); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("recurring", result.Recurring); err != nil {
		return diag.FromErr(err)
	}

	endpointIDs := make([]int, 0, len(result.Endpoints))
	for k := range result.Endpoints {
		if id, err := strconv.Atoi(k); err == nil {
			endpointIDs = append(endpointIDs, id)
		}
	}
	if err := d.Set("endpoints", endpointIDs); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceEdgeJobUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update edge job: %s", string(data)))
	}

	return nil
}

func resourceEdgeJobDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/edge_jobs/%s", client.Endpoint, d.Id()), nil)
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

	if resp.StatusCode != http.StatusNoContent {
		return diag.FromErr(fmt.Errorf("failed to delete edge job"))
	}

	return nil
}
