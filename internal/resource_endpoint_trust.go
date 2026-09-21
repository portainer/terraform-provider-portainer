package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceEndpointTrust grants trust to an edge environment sitting in
// Portainer's waiting room. The environment itself is created by the agent
// when it first calls home, not by Terraform, so this resource adopts an
// existing environment rather than creating one.
//
// Portainer's endpoint takes a batch, but one resource per environment is what
// lets Terraform track each one's trust state separately; the batch is sent as
// a list of one.
func resourceEndpointTrust() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointTrustCreate,
		ReadContext:   resourceEndpointTrustRead,
		// Portainer has no endpoint to revoke trust. Destroying the resource
		// stops managing it and leaves the environment trusted; untrusting is
		// done by deleting the environment.
		DeleteContext: removeFromStateContext,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the edge environment in the waiting room to trust. Use the `portainer_edge_waiting_room` data source to find environments awaiting trust.",
			},
			// The three relation fields below are only honoured by the call that
			// grants trust, so changing one has to re-run that call. They are
			// ForceNew rather than updatable in place: an update that quietly
			// did nothing would be worse than a plan that says what it will do.
			"group_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the endpoint group to place the environment in when trust is granted. Leave unset to keep the group the environment already has.",
			},
			"edge_group_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Identifiers of the edge groups to add the environment to when trust is granted. Leave unset to keep the edge groups the environment already has.",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Identifiers of the tags to apply to the environment when trust is granted. Leave unset to keep the tags the environment already has.",
			},
			// Computed attributes
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the trusted environment.",
			},
			"edge_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Edge identifier the agent reports for the environment.",
			},
			"trusted": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer reports the environment as trusted.",
			},
		},
	}
}

func resourceEndpointTrustCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	// Each relation field is omitted unless it was configured. Portainer reads
	// an empty array as "clear this", so sending one would strip the tags or
	// edge groups the environment already carries.
	relation := map[string]interface{}{}
	if v, ok := d.GetOk("group_id"); ok && v.(int) != 0 {
		relation["Group"] = v.(int)
	}
	if ids, ok := d.Get("edge_group_ids").([]interface{}); ok && len(ids) > 0 {
		relation["EdgeGroups"] = toIntSlice(ids)
	}
	if ids, ok := d.Get("tag_ids").([]interface{}); ok && len(ids) > 0 {
		relation["Tags"] = toIntSlice(ids)
	}

	payload := map[string]interface{}{"EndpointIDs": []int{endpointID}}
	if len(relation) > 0 {
		payload["Relations"] = map[string]interface{}{strconv.Itoa(endpointID): relation}
	}

	url := client.Endpoint + "/endpoints/edge/trust"
	if err := doJSON(ctx, client, http.MethodPost, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to trust environment %d: %w", endpointID, err))
	}

	d.SetId(strconv.Itoa(endpointID))
	return resourceEndpointTrustRead(ctx, d, meta)
}

func resourceEndpointTrustRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("the resource ID must be an environment identifier, got %q: %w", d.Id(), err))
	}

	// The relation fields are deliberately not read back. They are only
	// honoured when trust is granted, and reading a merged result into state
	// would make Terraform plan a replacement against its own configuration.
	var endpoint struct {
		ID          int    `json:"Id"`
		Name        string `json:"Name"`
		EdgeID      string `json:"EdgeID"`
		UserTrusted bool   `json:"UserTrusted"`
	}
	url := fmt.Sprintf("%s/endpoints/%d", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &endpoint); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read environment %d: %w", endpointID, err))
	}

	// An environment put back in the waiting room outside Terraform is drift
	// the next apply can actually fix, so it leaves state rather than being
	// reported as trusted.
	if !endpoint.UserTrusted {
		d.SetId("")
		return nil
	}

	if err := setFields(d, map[string]interface{}{
		"endpoint_id": endpoint.ID,
		"name":        endpoint.Name,
		"edge_id":     endpoint.EdgeID,
		"trusted":     endpoint.UserTrusted,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
