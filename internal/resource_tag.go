package internal

import (
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

	name := d.Get("name").(string)
	id, err := client.SDKClient.CreateTag(name)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	d.SetId(strconv.FormatInt(id, 10))
	return resourceTagRead(d, meta)
}

func resourceTagRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	tags, err := client.SDKClient.ListTags()
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	idStr := d.Id()
	for _, tag := range tags {
		if strconv.FormatInt(tag.ID, 10) == idStr {
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
