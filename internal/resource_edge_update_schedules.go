package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
		Create: resourcePortainerEdgeUpdateSchedulesCreate,
		Read:   resourcePortainerEdgeUpdateSchedulesRead,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true},
			"agent_image":    {Type: schema.TypeString, Required: true},
			"updater_image":  {Type: schema.TypeString, Required: true},
			"registry_id":    {Type: schema.TypeInt, Required: true},
			"scheduled_time": {Type: schema.TypeString, Required: true, Description: "Time in RFC3339 format"},
			"group_ids":      {Type: schema.TypeList, Required: true, Elem: &schema.Schema{Type: schema.TypeInt}},
			"type":           {Type: schema.TypeInt, Required: true, Description: "0 = update, 1 = rollback"},
		},
	}
}

func resourcePortainerEdgeUpdateSchedulesCreate(d *schema.ResourceData, meta interface{}) error {
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
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/edge_update_schedules", client.Endpoint), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create edge update schedule: %s", string(msg))
	}

	var response EdgeUpdateScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}
	d.SetId(strconv.Itoa(response.ID))
	return nil
}

func resourcePortainerEdgeUpdateSchedulesRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), nil)
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
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read edge update schedule: %s", string(msg))
	}

	var data EdgeUpdateScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}
	d.Set("name", data.Name)
	d.Set("agent_image", data.AgentImage)
	d.Set("updater_image", data.UpdaterImage)
	d.Set("registry_id", data.RegistryID)
	d.Set("scheduled_time", data.ScheduledTime)
	d.Set("group_ids", data.EdgeGroupIds)
	d.Set("type", data.Type)

	return nil
}

func resourcePortainerEdgeUpdateSchedulesUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	payload := map[string]interface{}{
		"name":          d.Get("name").(string),
		"agentImage":    d.Get("agent_image").(string),
		"updaterImage":  d.Get("updater_image").(string),
		"registryID":    d.Get("registry_id").(int),
		"scheduledTime": d.Get("scheduled_time").(string),
		"type":          d.Get("type").(int),
		"groupIDs":      d.Get("group_ids").([]interface{}),
	}

	// Convert []interface{} to []int explicitly
	groupIDsRaw := d.Get("group_ids").([]interface{})
	groupIDs := make([]int, len(groupIDsRaw))
	for i, v := range groupIDsRaw {
		groupIDs[i] = v.(int)
	}
	payload["groupIDs"] = groupIDs

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update edge update schedule: %s", string(msg))
	}

	return nil
}

func resourcePortainerEdgeUpdateSchedulesDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/edge_update_schedules/%s", client.Endpoint, id), nil)
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
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete edge update schedule: %s", string(msg))
	}

	d.SetId("")
	return nil
}
