package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
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

	params := users.NewUserListParams()
	resp, err := client.Client.Users.UserList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range resp.Payload {
		if u.Username == username {
			d.SetId(strconv.FormatInt(u.ID, 10))
			d.Set("role", u.Role)
			return nil
		}
	}

	return fmt.Errorf("user %s not found", username)
}
