package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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
		CreateContext: resourceCloudProvisionCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"cloud_provider": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Cloud provider (civo, digitalocean, linode, amazon, azure, gke)",
				ValidateFunc: validation.StringInSlice([]string{
					"civo", "digitalocean", "linode", "amazon", "azure", "gke",
				}, false),
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

func resourceCloudProvisionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	provider := d.Get("cloud_provider").(string)

	timeout := d.Timeout(schema.TimeoutCreate)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := mapStringInterfaceCloudProviderProvision(d.Get("payload").(map[string]interface{}))

	url := fmt.Sprintf("%s/cloud/%s/provision", client.Endpoint, provider)
	var result struct {
		Id int `json:"Id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("cloud provision failed: %w", err))
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
