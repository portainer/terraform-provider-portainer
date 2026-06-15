package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceEdgeGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEdgeGroupCreate,
		ReadContext:   resourceEdgeGroupRead,
		DeleteContext: resourceEdgeGroupDelete,
		UpdateContext: resourceEdgeGroupUpdate,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Portainer Edge group.",
			},
			"dynamic": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether the Edge group is dynamic (membership determined by tags) or static (explicit endpoints).",
			},
			"partial_match": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "For dynamic groups, whether to match endpoints with any (true) or all (false) of the specified tags.",
			},
			"endpoints": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Optional:    true,
				Description: "List of Portainer environment (endpoint) IDs that are statically assigned to the Edge group. Used when dynamic is false.",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Optional:    true,
				Description: "List of tag IDs used to dynamically select endpoints. Used when dynamic is true.",
			},
		},
	}
}

func findExistingEdgeGroupByName(ctx context.Context, client *APIClient, name string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_groups", client.Endpoint), nil)
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
		return 0, fmt.Errorf("failed to list edge groups: %s", string(data))
	}

	var groups []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		return 0, err
	}

	for _, g := range groups {
		if g["Name"] == name {
			if id, ok := g["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}

	return 0, nil
}

func resourceEdgeGroupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	if existingID, err := findExistingEdgeGroupByName(ctx, client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing edge group: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEdgeGroupUpdate(ctx, d, meta)
	}

	payload := buildEdgeGroupPayload(d)
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_groups", client.Endpoint), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create edge group: %s", string(data)))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceEdgeGroupRead(ctx, d, meta)
}

func resourceEdgeGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_groups/%s", client.Endpoint, d.Id()), nil)
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

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	} else if resp.StatusCode != http.StatusOK {
		return diag.FromErr(fmt.Errorf("failed to read edge group"))
	}

	var group struct {
		Name         string `json:"Name"`
		Dynamic      bool   `json:"Dynamic"`
		PartialMatch bool   `json:"PartialMatch"`
		TagIDs       []int  `json:"TagIds"`
		Endpoints    []int  `json:"Endpoints"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&group); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", group.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("dynamic", group.Dynamic); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("partial_match", group.PartialMatch); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("tag_ids", group.TagIDs); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("endpoints", group.Endpoints); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceEdgeGroupUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := buildEdgeGroupPayload(d)
	jsonBody, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/edge_groups/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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
		return diag.FromErr(fmt.Errorf("failed to update edge group: %s", string(data)))
	}

	return resourceEdgeGroupRead(ctx, d, meta)
}

func resourceEdgeGroupDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/edge_groups/%s", client.Endpoint, d.Id()), nil)
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
		return diag.FromErr(fmt.Errorf("failed to delete edge group"))
	}

	return nil
}

func buildEdgeGroupPayload(d *schema.ResourceData) map[string]interface{} {
	payload := map[string]interface{}{
		"name":         d.Get("name").(string),
		"dynamic":      d.Get("dynamic").(bool),
		"partialMatch": d.Get("partial_match").(bool),
	}

	if v, ok := d.GetOk("endpoints"); ok {
		payload["endpoints"] = v
	}
	if v, ok := d.GetOk("tag_ids"); ok {
		payload["tagIDs"] = v
	}

	return payload
}
