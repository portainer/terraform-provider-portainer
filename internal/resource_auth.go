package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAuth() *schema.Resource {
	return &schema.Resource{
		Create: resourceAuthCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"username": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"jwt": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

func resourceAuthCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	creds := map[string]string{
		"username": d.Get("username").(string),
		"password": d.Get("password").(string),
	}

	resp, err := client.DoRequest("POST", "/auth", nil, creds)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to authenticate: %s", string(data))
	}

	var response struct {
		JWT string `json:"jwt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	d.SetId("auth-result")
	d.Set("jwt", response.JWT)

	return nil
}
