package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceUser() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceUserRead,

		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Username of the Portainer user to look up. The data source will fail if no matching user is found.",
			},
			"role": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Role assigned to the user in Portainer: 1 = admin, 2 = user.",
			},
		},
	}
}

func dataSourceUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	username := d.Get("username").(string)

	userList, err := listUsers(ctx, client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list users: %w", err))
	}

	for _, u := range userList {
		if u.Username == username {
			d.SetId(strconv.FormatInt(u.ID, 10))
			if err := d.Set("role", u.Role); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("user %s not found", username))
}
