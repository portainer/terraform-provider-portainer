package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointServiceUpdate() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointServiceUpdateExecute,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"service_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"pull_image": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  false,
			},
		},
	}
}

func resourceEndpointServiceUpdateExecute(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	serviceName := d.Get("service_name").(string)
	pullImage := d.Get("pull_image").(bool)

	serviceID, err := resolveServiceID(client, endpointID, serviceName)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"pullImage": pullImage,
		"serviceID": serviceID,
	}
	jsonBody, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/endpoints/%d/forceupdateservice", client.Endpoint, endpointID)
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update service: %s", string(body))
	}

	warnings := struct {
		Warnings []string `json:"Warnings"`
	}{}
	_ = json.NewDecoder(resp.Body).Decode(&warnings)
	if len(warnings.Warnings) > 0 {
		fmt.Printf("[WARN] Service update warnings: %v\n", warnings.Warnings)
	}

	d.SetId(strconv.Itoa(endpointID) + "-" + serviceID)
	return nil
}

func resolveServiceID(client *APIClient, endpointID int, name string) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/services", client.Endpoint, endpointID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch services: %s", string(body))
	}

	var services []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return "", err
	}

	for _, service := range services {
		if service.Spec.Name == name {
			return service.ID, nil
		}
	}

	return "", fmt.Errorf("service with name '%s' not found", name)
}
