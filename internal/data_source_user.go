package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceUser() *schema.Resource {
	r := &schema.Resource{
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
	for k, v := range userAuthenticationMethodSchema() {
		r.Schema[k] = v
	}
	return r
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
			// The list leaves out the 3.0 authentication method fields;
			// the user's own endpoint has them.
			var detail portainerUser
			if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/users/%d", client.Endpoint, u.ID), nil, &detail); err == nil {
				u.CreatedAuthenticationMethod = detail.CreatedAuthenticationMethod
				u.LastAuthenticationMethod = detail.LastAuthenticationMethod
			}
			if err := setUserAuthenticationMethods(d, u); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("user %s not found", username))
}
