package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceResourceControl() *schema.Resource {
	return &schema.Resource{
		Create: resourceResourceControlCreate,
		Read:   resourceResourceControlRead,
		Update: resourceResourceControlUpdate,
		Delete: resourceResourceControlDelete,

		Schema: map[string]*schema.Schema{
			"resource_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"sub_resource_ids": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"administrators_only": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"public": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"teams": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"users": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func resourceResourceControlCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	body := map[string]interface{}{
		"resourceID":         d.Get("resource_id").(string),
		"subResourceIDs":     d.Get("sub_resource_ids"),
		"type":               d.Get("type").(int),
		"administratorsOnly": d.Get("administrators_only").(bool),
		"public":             d.Get("public").(bool),
		"teams":              d.Get("teams"),
		"users":              d.Get("users"),
	}

	resp, err := client.DoRequest("POST", "/resource_controls", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create resource control: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	d.SetId(strconv.Itoa(result.ID))
	return resourceResourceControlRead(d, meta)
}

func resourceResourceControlRead(d *schema.ResourceData, meta interface{}) error {
	// Optional: implement GET if supported
	return nil
}

func resourceResourceControlUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	body := map[string]interface{}{
		"administratorsOnly": d.Get("administrators_only").(bool),
		"public":             d.Get("public").(bool),
		"teams":              d.Get("teams"),
		"users":              d.Get("users"),
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/resource_controls/%s", id), nil, body)
	if err != nil {
		return fmt.Errorf("failed to update resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update resource control: %s", string(data))
	}

	return resourceResourceControlRead(d, meta)
}

func resourceResourceControlDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/resource_controls/%s", id), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete resource control: %s", string(data))
	}

	d.SetId("")
	return nil
}
