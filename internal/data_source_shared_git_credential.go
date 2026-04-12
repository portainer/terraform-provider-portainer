package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePortainerSharedGitCredential() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePortainerSharedGitCredentialRead,

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

func dataSourcePortainerSharedGitCredentialRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/cloud/gitcredentials", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list shared git credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list shared git credentials, status %d: %s", resp.StatusCode, string(data))
	}

	var credentials []struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		return fmt.Errorf("failed to decode shared git credentials list: %w", err)
	}

	name := d.Get("name").(string)

	for _, c := range credentials {
		if c.Name == name {
			d.SetId(strconv.Itoa(c.ID))
			_ = d.Set("name", c.Name)
			_ = d.Set("username", c.Username)
			_ = d.Set("authorization_type", c.AuthorizationType)
			_ = d.Set("user_id", c.UserID)
			return nil
		}
	}

	return fmt.Errorf("shared git credential with name %q not found", name)
}
