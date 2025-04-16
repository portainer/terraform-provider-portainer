package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceTeam() *schema.Resource {
	return &schema.Resource{
		Create: resourceTeamCreate,
		Read:   resourceTeamRead,
		Delete: resourceTeamDelete,
		Update: resourceTeamUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceTeamCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	body := map[string]interface{}{
		"Name": teamName,
	}

	resp, err := client.DoRequest("POST", "/teams", nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create team: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceTeamRead(d, meta)
}

func resourceTeamRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", fmt.Sprintf("/teams/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("failed to read team: status %d", resp.StatusCode)
	}

	var result struct {
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("name", result.Name)
	return nil
}

func resourceTeamUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	body := map[string]interface{}{
		"name": d.Get("name").(string),
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/teams/%s", d.Id()), nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update team: %s", string(data))
	}

	return resourceTeamRead(d, meta)
}

func resourceTeamDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/teams/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 204 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete team: %s", string(data))
}
