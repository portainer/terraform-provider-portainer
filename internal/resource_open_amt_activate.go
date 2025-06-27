package internal

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerOpenAMTActivate() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerOpenAMTActivateCreate,
		Read:   schema.Noop,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The ID of the environment (endpoint) to activate OpenAMT on.",
			},
		},
	}
}

func resourcePortainerOpenAMTActivateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	url := fmt.Sprintf("%s/open_amt/%d/activate", client.Endpoint, id)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to activate OpenAMT: %s", resp.Status)
	}

	d.SetId("openamt-" + strconv.Itoa(id))
	return nil
}
