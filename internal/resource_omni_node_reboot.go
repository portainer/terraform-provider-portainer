package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceOmniNodeReboot reboots a node of an Omni cluster. A reboot is a
// one-shot action with no state to read back, so the resource records that it
// ran and `triggers` is how another one is asked for.
func resourceOmniNodeReboot() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceOmniNodeRebootCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"credential_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
			},
			"cluster": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the cluster the node belongs to.",
			},
			"node": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the node to reboot.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another reboot when they change. A reboot is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceOmniNodeRebootCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	credentialID := d.Get("credential_id").(int)
	cluster := d.Get("cluster").(string)
	node := d.Get("node").(string)

	rebootURL := fmt.Sprintf("%s/omni/%d/node/%s?cluster=%s",
		client.Endpoint, credentialID, url.PathEscape(node), url.QueryEscape(cluster))
	if err := doJSON(ctx, client, http.MethodPost, rebootURL, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to reboot node %s of Omni cluster %s: %w", node, cluster, err))
	}

	d.SetId(fmt.Sprintf("%d/%s/%s/reboot-%d", credentialID, cluster, node, makeTimestamp()))
	return nil
}
