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
				Type:     schema.TypeString,
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

func toIntSlices(raw []interface{}) []int {
	result := []int{}
	for _, v := range raw {
		result = append(result, v.(int))
	}
	return result
}

// getNamespaceRPN resolves namespace name against the Portainer API.
// Even though API objects have an Id, the /pools/{rpn}/access endpoint
// only works with the namespace name, so we must return Name.
func getNamespaceRPN(client *APIClient, environmentID int, namespaceName string) (string, error) {
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces", client.Endpoint, environmentID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to list namespaces: %s", string(data))
	}

	var namespaces []struct {
		Name string `json:"Name"`
		Id   string `json:"Id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&namespaces); err != nil {
		return "", err
	}

	for _, ns := range namespaces {
		if ns.Name == namespaceName {
			return ns.Name, nil
		}
	}

	return "", fmt.Errorf("namespace %s not found", namespaceName)
}

func resourceK8sAccessUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespaceID := d.Get("namespace_id").(string)

	if !containsColon(namespaceID) {
		nsName, err := getNamespaceRPN(client, endpointID, namespaceID)
		if err != nil {
			return err
		}
		namespaceID = nsName
	}

	body := map[string]interface{}{
		"usersToAdd":    toIntSlices(d.Get("users_to_add").([]interface{})),
		"usersToRemove": toIntSlices(d.Get("users_to_remove").([]interface{})),
		"teamsToAdd":    toIntSlices(d.Get("teams_to_add").([]interface{})),
		"teamsToRemove": toIntSlices(d.Get("teams_to_remove").([]interface{})),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/endpoints/%d/pools/%s/access", client.Endpoint, endpointID, namespaceID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
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

	d.SetId(fmt.Sprintf("%d/%s", endpointID, namespaceID))
	return nil
}

func resourceK8sAccessReadNoop(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceK8sAccessDeleteNoop(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
