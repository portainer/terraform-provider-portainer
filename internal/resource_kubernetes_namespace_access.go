package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceAccess() *schema.Resource {
	return &schema.Resource{
		Create: resourceK8sAccessUpdate,
		Read:   resourceK8sAccessReadNoop,
		Update: resourceK8sAccessUpdate,
		Delete: resourceK8sAccessDeleteNoop,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"namespace_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"users_to_add": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"users_to_remove": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"teams_to_add": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"teams_to_remove": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func resourceK8sAccessUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespaceID := d.Get("namespace_id").(int)

	body := map[string]interface{}{
		"usersToAdd":    toIntSlice(d.Get("users_to_add").([]interface{})),
		"usersToRemove": toIntSlice(d.Get("users_to_remove").([]interface{})),
		"teamsToAdd":    toIntSlice(d.Get("teams_to_add").([]interface{})),
		"teamsToRemove": toIntSlice(d.Get("teams_to_remove").([]interface{})),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/endpoints/%d/pools/%d/access", client.Endpoint, endpointID, namespaceID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update namespace access: %s", string(data))
	}

	d.SetId(fmt.Sprintf("%d/%d", endpointID, namespaceID))
	return nil
}

func resourceK8sAccessReadNoop(d *schema.ResourceData, meta interface{}) error {
	// No reliable read endpoint exists for this resource
	return nil
}

func resourceK8sAccessDeleteNoop(d *schema.ResourceData, meta interface{}) error {
	// No delete behavior – access must be manually revoked or redefined
	return nil
}
