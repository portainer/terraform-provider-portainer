package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerSSHKeygen() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerSSHKeygenCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,
		Schema: map[string]*schema.Schema{
			"public": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Generated public key",
			},
			"private": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "Generated private key",
			},
		},
	}
}

func resourcePortainerSSHKeygenCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/sshkeygen", client.Endpoint), nil)
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

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to generate SSH key: %s", string(body)))
	}

	var keypair struct {
		Public  string `json:"public"`
		Private string `json:"private"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&keypair); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode response: %w", err))
	}

	if err := d.Set("public", keypair.Public); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("private", keypair.Private); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(strconv.Itoa(len(keypair.Public)))

	return nil
}
