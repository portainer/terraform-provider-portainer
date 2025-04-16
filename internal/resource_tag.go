package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceTag() *schema.Resource {
	return &schema.Resource{
		Create: resourceTagCreate,
		Read:   resourceTagRead,
		Delete: resourceTagDelete,
		Update: nil,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceTagCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name": d.Get("name").(string),
	}

	resp, err := client.DoRequest("POST", "/tags", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create tag: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceTagRead(d, meta)
}

func resourceTagRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", fmt.Sprintf("/tags/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode == 200 {
		var tag struct {
			Name string `json:"Name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tag); err == nil && tag.Name != "" {
			d.Set("name", tag.Name)
			return nil
		}
	}

	respList, err := client.DoRequest("GET", "/tags", nil, nil)
	if err != nil {
		return err
	}
	defer respList.Body.Close()

	if respList.StatusCode != 200 {
		return fmt.Errorf("failed to fallback to GET /tags list")
	}

	var tags []struct {
		ID   int    `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(respList.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to decode fallback tag list: %s", err)
	}

	for _, tag := range tags {
		if strconv.Itoa(tag.ID) == d.Id() {
			d.Set("name", tag.Name)
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceTagDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/tags/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete tag: %s", string(data))
}
