package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceGitopsSourceConnection() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsSourceConnectionRead,

		Schema: map[string]*schema.Schema{
			"source_id": {
				Type:          schema.TypeInt,
				Optional:      true,
				ConflictsWith: []string{"url"},
				Description:   "Identifier of a stored GitOps source to test. Mutually exclusive with `url`; exactly one of the two is required.",
			},
			"url": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"source_id"},
				Description:   "Repository URL to test without storing a source, for validating credentials before creating one. Mutually exclusive with `source_id`.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username used for the test. Leave unset for an anonymous repository.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password or personal access token paired with `username`.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip TLS certificate verification during the test.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether a failing connection makes the data source itself fail. Defaults to false, which reports the outcome in `success` and `error`.",
			},
			// Computed attributes
			"success": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach and authenticate against the repository.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Reason the connection failed, empty on success.",
			},
		},
	}
}

func dataSourceGitopsSourceConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	sourceID, hasID := d.GetOk("source_id")
	repoURL, hasURL := d.GetOk("url")
	if hasID == hasURL {
		return diag.FromErr(fmt.Errorf("exactly one of source_id or url must be set"))
	}

	payload := map[string]interface{}{"tlsSkipVerify": d.Get("tls_skip_verify").(bool)}
	if auth := gitopsAuthPayload(d); auth != nil {
		payload["authentication"] = auth
	}

	// Portainer exposes two test endpoints: one for an ad-hoc repository and one
	// that re-tests a stored source, the latter taking the update payload so the
	// credentials of the stored source can be overridden for the test.
	var endpoint, id string
	if hasID {
		id = strconv.Itoa(sourceID.(int))
		endpoint = fmt.Sprintf("%s/gitops/sources/%s/test", client.Endpoint, id)
	} else {
		payload["url"] = repoURL.(string)
		endpoint = client.Endpoint + "/gitops/sources/test"
		id = repoURL.(string)
	}

	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := doJSON(ctx, client, http.MethodPost, endpoint, payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to test the GitOps source connection: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"success": result.Success,
		"error":   result.Error,
	}); err != nil {
		return diag.FromErr(err)
	}

	// The request itself succeeded even when the repository is unreachable, so
	// the ID is set before the opt-in failure below.
	d.SetId("gitops-source-test-" + id)

	if !result.Success && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the GitOps source connection test failed: %s", result.Error))
	}

	return nil
}
