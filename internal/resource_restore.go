package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRestore() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRestoreCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"file_content_base64": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"file_content_base64", "file_content_base64_wo"},
				ForceNew:     true,
				Sensitive:    true,
				Description:  "Base64-encoded content of the backup archive, typically `filebase64(\"portainer-backup.tar.gz\")`. A backup contains every credential Portainer holds, so it is stored in state as a sensitive value; use `file_content_base64_wo` to keep it out of state.",
			},
			"file_content_base64_wo":         writeOnlySecretSchema("file_content_base64", "file_content_base64_wo_version", "base64-encoded backup archive"),
			"file_content_base64_wo_version": writeOnlyVersionSchema("file_content_base64_wo", true),
			"file_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "portainer-backup.tar.gz",
				ForceNew:    true,
				Description: "Name reported to Portainer for the uploaded archive.",
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Password the backup was encrypted with. Leave unset for an unencrypted backup. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
				ConflictsWith: []string{"password_wo"},
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password the backup archive was encrypted with"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", true),
			// Write-only only: the token is a one-time bootstrap secret, so there
			// is no plaintext twin that would put it in state.
			"setup_token_wo": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				WriteOnly:    true,
				RequiredWith: []string{"setup_token_wo_version"},
				Description:  "Write-only value sent as the `X-Setup-Token` header (supports ephemeral values; never stored in Terraform state). Portainer requires it when the instance has not been initialised yet, which is the usual state for a restore. Requires Terraform 1.11 or later.",
			},
			"setup_token_wo_version": writeOnlyVersionSchema("setup_token_wo", true),
		},
	}
}

func resourceRestoreCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	encoded, err := twinSecret(d, "file_content_base64")
	if err != nil {
		return diag.FromErr(err)
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return diag.FromErr(fmt.Errorf("file_content_base64 is not valid base64: %w", err))
	}
	if len(content) == 0 {
		return diag.FromErr(fmt.Errorf("file_content_base64 decodes to an empty archive"))
	}

	// FileContent is []byte on the Portainer side, so encoding/json re-encodes it
	// as base64 on the wire; the decode above is what validates the input.
	payload := map[string]interface{}{
		"FileContent": content,
		"FileName":    d.Get("file_name").(string),
	}
	password, err := twinSecret(d, "password")
	if err != nil {
		return diag.FromErr(err)
	}
	if password != "" {
		payload["Password"] = password
	}

	// The setup token goes in a header rather than the body, so this call does
	// not go through doJSON.
	headers := map[string]string{}
	if d.Get("setup_token_wo_version").(int) != 0 {
		setupToken, err := writeOnlyString(d, "setup_token_wo")
		if err != nil {
			return diag.FromErr(err)
		}
		if setupToken != "" {
			headers["X-Setup-Token"] = setupToken
		}
	}
	if err := doJSONWithHeaders(ctx, client, http.MethodPost, client.Endpoint+"/restore", payload, nil, headers); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore the Portainer backup: %w", err))
	}

	d.SetId("portainer-restore-" + fmt.Sprint(makeTimestamp()))
	return nil
}
