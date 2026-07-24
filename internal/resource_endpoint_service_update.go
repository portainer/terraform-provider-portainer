package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEndpointServiceUpdate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointServiceUpdateExecute,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer endpoint hosting the Swarm service to force-update.",
			},
			"service_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Swarm service that should be force-updated.",
			},
			"pull_image": {
				Type:        schema.TypeBool,
				Optional:    true,
				ForceNew:    true,
				Default:     false,
				Description: "Whether Portainer should pull the latest image when force-updating the service.",
			},
		},
	}
}

func resourceEndpointServiceUpdateExecute(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	serviceName := d.Get("service_name").(string)
	pullImage := d.Get("pull_image").(bool)

	serviceID, err := resolveServiceID(ctx, client, endpointID, serviceName)
	if err != nil {
		return diag.FromErr(err)
	}

	payload := map[string]interface{}{
		"pullImage": pullImage,
		"serviceID": serviceID,
	}

	url := fmt.Sprintf("%s/endpoints/%d/forceupdateservice", client.Endpoint, endpointID)
	warnings := struct {
		Warnings []string `json:"Warnings"`
	}{}
	if err := doJSON(ctx, client, http.MethodPut, url, payload, &warnings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update service: %w", err))
	}

	if len(warnings.Warnings) > 0 {
		fmt.Printf("[WARN] Service update warnings: %v\n", warnings.Warnings)
	}

	d.SetId(strconv.Itoa(endpointID) + "-" + serviceID)
	return nil
}

func resolveServiceID(ctx context.Context, client *APIClient, endpointID int, name string) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/services", client.Endpoint, endpointID)
	var services []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &services); err != nil {
		return "", fmt.Errorf("failed to fetch services: %w", err)
	}

	for _, service := range services {
		if service.Spec.Name == name {
			return service.ID, nil
		}
	}

	return "", fmt.Errorf("service with name '%s' not found", name)
}
