package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
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

	ctx, errBody := withErrorCapture(ctx)
	params := users.NewUserListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Users.UserList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list users: %w", decorateSDKError(err, errBody)))
	}

	for _, u := range resp.Payload {
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
