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

func dataSourceDockerSecret() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerSecretRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer endpoint hosting the Docker secret.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Docker secret to look up.",
			},
		},
	}
}

func dataSourceDockerSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/secrets", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker secrets: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to list docker secrets, status %d: %s", resp.StatusCode, string(data)))
	}

	var secrets []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker secret list: %w", err))
	}

	for _, s := range secrets {
		if s.Spec.Name == name {
			d.SetId(s.ID)
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("docker secret %s not found in endpoint %d", name, endpointID))
}
