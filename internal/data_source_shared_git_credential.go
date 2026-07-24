package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePortainerSharedGitCredential() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcePortainerSharedGitCredentialRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the shared git credential to look up",
			},
			"username": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Username for git authentication",
			},
			"authorization_type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Authorization type: 0 = Basic, 1 = Token",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "User ID of the credential owner",
			},
		},
	}
}

func dataSourcePortainerSharedGitCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var credentials []struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/cloud/gitcredentials", nil, &credentials); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list shared git credentials: %w", err))
	}

	name := d.Get("name").(string)

	for _, c := range credentials {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			if err := setFields(d, map[string]interface{}{
				"name":               c.Name,
				"username":           c.Username,
				"authorization_type": c.AuthorizationType,
				"user_id":            c.UserID,
			}); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("shared git credential with name %q not found", name))
}
