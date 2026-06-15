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

type EdgeUpdateSchedulePayload struct {
	Name          string `json:"name"`
	AgentImage    string `json:"agentImage"`
	UpdaterImage  string `json:"updaterImage"`
	RegistryID    int    `json:"registryID"`
	ScheduledTime string `json:"scheduledTime"`
	GroupIDs      []int  `json:"groupIDs"`
	Type          int    `json:"type"`
}

type EdgeUpdateScheduleResponse struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	AgentImage    string `json:"agentImage"`
	UpdaterImage  string `json:"updaterImage"`
	RegistryID    int    `json:"registryId"`
	ScheduledTime string `json:"scheduledTime"`
	EdgeGroupIds  []int  `json:"edgeGroupIds"`
	Type          int    `json:"type"`
	Status        int    `json:"status"`
	StatusMessage string `json:"statusMessage"`
	Version       string `json:"version"`
}

func resourcePortainerEdgeUpdateSchedules() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerEdgeUpdateSchedulesCreate,
		ReadContext:   resourcePortainerEdgeUpdateSchedulesRead,
		UpdateContext: resourcePortainerEdgeUpdateSchedulesUpdate,
		DeleteContext: resourcePortainerEdgeUpdateSchedulesDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true, ValidateFunc: validation.NoZeroValues, Description: "Name of the Portainer edge update schedule."},
			"agent_image":    {Type: schema.TypeString, Required: true, Description: "Container image used as the target Portainer Edge agent image after the update runs."},
			"updater_image":  {Type: schema.TypeString, Required: true, Description: "Container image used to perform the Edge agent update."},
			"registry_id":    {Type: schema.TypeInt, Required: true, Description: "Identifier of the Portainer registry from which the agent and updater images are pulled."},
			"scheduled_time": {Type: schema.TypeString, Required: true, Description: "Time in RFC3339 format"},
			"group_ids":      {Type: schema.TypeList, Required: true, Elem: &schema.Schema{Type: schema.TypeInt}, Description: "List of edge group identifiers targeted by the update schedule."},
			"type":           {Type: schema.TypeInt, Required: true, Description: "0 = update, 1 = rollback", ValidateFunc: validation.IntBetween(0, 1)},
		},
	}
}

func resourcePortainerEdgeUpdateSchedulesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := EdgeUpdateSchedulePayload{
		Name:          d.Get("name").(string),
		AgentImage:    d.Get("agent_image").(string),
		UpdaterImage:  d.Get("updater_image").(string),
		RegistryID:    d.Get("registry_id").(int),
		ScheduledTime: d.Get("scheduled_time").(string),
		Type:          d.Get("type").(int),
	}

	for _, id := range d.Get("group_ids").([]interface{}) {
		payload.GroupIDs = append(payload.GroupIDs, id.(int))
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_update_schedules", client.Endpoint), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create edge update schedule: %s", string(msg)))
	}

	var response EdgeUpdateScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(strconv.Itoa(response.ID))
	return nil
}

func resourcePortainerEdgeUpdateSchedulesUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	payload := EdgeUpdateSchedulePayload{
		Name:          d.Get("name").(string),
		AgentImage:    d.Get("agent_image").(string),
		UpdaterImage:  d.Get("updater_image").(string),
		RegistryID:    d.Get("registry_id").(int),
		ScheduledTime: d.Get("scheduled_time").(string),
		Type:          d.Get("type").(int),
	}

	for _, gid := range d.Get("group_ids").([]interface{}) {
		payload.GroupIDs = append(payload.GroupIDs, gid.(int))
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update edge update schedule: %s", string(msg)))
	}

	return resourcePortainerEdgeUpdateSchedulesRead(ctx, d, meta)
}

func resourcePortainerEdgeUpdateSchedulesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), nil)
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

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete edge update schedule: %s", string(msg)))
	}

	return nil
}

func resourcePortainerEdgeUpdateSchedulesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), nil)
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
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read edge update schedule: %s", string(msg)))
	}

	var data EdgeUpdateScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", data.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("agent_image", data.AgentImage); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("updater_image", data.UpdaterImage); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("registry_id", data.RegistryID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("scheduled_time", data.ScheduledTime); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("group_ids", data.EdgeGroupIds); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", data.Type); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
