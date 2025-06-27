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

type CloudProvisionPayload struct {
	CredentialID      int                    `json:"credentialID"`
	Name              string                 `json:"name"`
	Region            string                 `json:"region"`
	NodeCount         int                    `json:"nodeCount"`
	NodeSize          string                 `json:"nodeSize"`
	NetworkID         string                 `json:"networkID"`
	KubernetesVersion string                 `json:"kubernetesVersion"`
	InstanceType      string                 `json:"instanceType,omitempty"`
	AmiType           string                 `json:"amiType,omitempty"`
	NodeVolumeSize    int                    `json:"nodeVolumeSize,omitempty"`
	DnsPrefix         string                 `json:"dnsPrefix,omitempty"`
	ResourceGroup     string                 `json:"resourceGroup,omitempty"`
	ResourceGroupName string                 `json:"resourceGroupName,omitempty"`
	PoolName          string                 `json:"poolName,omitempty"`
	AvailabilityZones []string               `json:"availabilityZones,omitempty"`
	Tier              string                 `json:"tier,omitempty"`
	CPU               int                    `json:"cpu,omitempty"`
	RAM               int                    `json:"ram,omitempty"`
	HDD               int                    `json:"hdd,omitempty"`
	Meta              map[string]interface{} `json:"meta,omitempty"`
}

func resourcePortainerCloudProvision() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudProvisionCreate,
		Read:   schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"cloud_provider": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Cloud provider (civo, digitalocean, linode, amazon, azure, gke)",
			},
			"payload": {
				Type:        schema.TypeMap,
				Required:    true,
				ForceNew:    true,
				Description: "Raw payload with provisioning parameters.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceCloudProvisionCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	provider := d.Get("cloud_provider").(string)

	payload := mapStringInterfaceCloudProviderProvision(d.Get("payload").(map[string]interface{}))
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/cloud/%s/provision", client.Endpoint, provider)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
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
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloud provision failed: %s", msg)
	}

	var result struct {
		Id int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	d.SetId(strconv.Itoa(result.Id))
	return nil
}

func mapStringInterfaceCloudProviderProvision(input map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range input {
		out[k] = v
	}
	return out
}
