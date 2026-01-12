package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceUser() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceUserRead,

		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeString,
				Required: true,
			},
			"role": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceUserRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	username := d.Get("username").(string)

	resp, err := client.DoRequest("GET", "/users", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list users, status %d: %s", resp.StatusCode, string(data))
	}

	var users []struct {
		ID       int    `json:"Id"`
		Username string `json:"Username"`
		Role     int    `json:"Role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return fmt.Errorf("failed to decode user list: %w", err)
	}

	for _, u := range users {
		if u.Username == username {
			d.SetId(strconv.Itoa(u.ID))
			d.Set("role", u.Role)
			return nil
		}
	}

	return fmt.Errorf("user %s not found", username)
}
