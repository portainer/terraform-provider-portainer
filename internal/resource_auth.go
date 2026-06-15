package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAuth() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAuthCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Username used to authenticate against the Portainer API. Stored in state as a sensitive value. Changing this value forces resource recreation.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Password used to authenticate against the Portainer API. Stored in state as a sensitive value. Changing this value forces resource recreation.",
			},
			"jwt": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "JWT bearer token issued by Portainer after successful authentication. Computed and stored in state as a sensitive value.",
			},
		},
	}
}

func resourceAuthCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	creds := map[string]string{
		"username": d.Get("username").(string),
		"password": d.Get("password").(string),
	}

	resp, err := client.DoRequest("POST", "/auth", nil, creds)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to authenticate: %s", string(data)))
	}

	var response struct {
		JWT string `json:"jwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("auth-result")
	if err := d.Set("jwt", response.JWT); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
