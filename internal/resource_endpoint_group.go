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

func resourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointGroupCreate,
		Read:   resourceEndpointGroupRead,
		Delete: resourceEndpointGroupDelete,
		Update: resourceEndpointGroupUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"tag_ids": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func resourceEndpointGroupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name": d.Get("name").(string),
	}

	if v, ok := d.GetOk("description"); ok {
		payload["description"] = v.(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, id.(int))
		}
		payload["tagIDs"] = tagIDs
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/endpoint_groups", client.Endpoint), bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to create endpoint group: %s", string(data))
	}

	var result struct {
		ID          int    `json:"Id"`
		Description string `json:"Description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceEndpointGroupRead(d, meta)
}

func resourceEndpointGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/endpoint_groups/%s", client.Endpoint, d.Id()), nil)
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to read endpoint group")
	}

	var group struct {
		Name        string `json:"Name"`
		Description string `json:"Description"`
		TagIDs      []int  `json:"TagIds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&group); err != nil {
		return err
	}

	d.Set("name", group.Name)
	d.Set("description", group.Description)
	d.Set("tag_ids", group.TagIDs)

	return nil
}

func resourceEndpointGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name": d.Get("name").(string),
	}

	if v, ok := d.GetOk("description"); ok {
		payload["description"] = v.(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, id.(int))
		}
		payload["tagIDs"] = tagIDs
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/endpoint_groups/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to update endpoint group: %s", string(data))
	}

	return resourceEndpointGroupRead(d, meta)
}

func resourceEndpointGroupDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/endpoint_groups/%s", client.Endpoint, d.Id()), nil)
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete endpoint group: %s", string(data))
}
