package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// k8sConfirmExistsByGET issues a GET against a manifest-based Kubernetes resource to
// verify it still exists. On HTTP 404 it clears the resource ID via d.SetId("") so the
// next plan recreates it (out-of-band deletion detection). On any other non-2xx it
// returns diagnostics carrying the response body.
//
// It deliberately does NOT refresh the authored "manifest" field. The Kubernetes API
// returns a fully server-expanded object (status, managedFields, resourceVersion,
// defaulted spec fields, …) that never matches the user's hand-written manifest;
// writing it back would produce a permanent diff. Since these resources' Update is
// delete+recreate, that diff would also churn the workload on every apply. So the
// authored manifest stays the source of truth in state — Read only confirms existence
// and (in the caller) recovers identity fields. Manifest-content drift is therefore
// not detected; only deletion is. After `terraform import`, the manifest field must be
// set in config to match the live object.
//
// Returns nil diagnostics with the ID preserved when the object exists, or nil
// diagnostics with the ID cleared when it is gone. Callers should check d.Id() == ""
// before setting any other fields.
func k8sConfirmExistsByGET(ctx context.Context, d *schema.ResourceData, client *APIClient, url, kind string) diag.Diagnostics {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read %s (%d): %s", kind, resp.StatusCode, string(body)))
	}
	return nil
}
