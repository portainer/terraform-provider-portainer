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
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Base64-encoded content of the backup archive, typically `filebase64(\"portainer-backup.tar.gz\")`. A backup contains every credential Portainer holds, so it is stored in state as a sensitive value.",
			},
			"file_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "portainer-backup.tar.gz",
				ForceNew:    true,
				Description: "Name reported to Portainer for the uploaded archive.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Password the backup was encrypted with. Leave unset for an unencrypted backup.",
			},
			"setup_token": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Value sent as the `X-Setup-Token` header. Portainer requires it when the instance has not been initialised yet, which is the usual state for a restore.",
			},
		},
	}
}

func resourceRestoreCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	encoded := d.Get("file_content_base64").(string)
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
	if v, ok := d.GetOk("password"); ok {
		payload["Password"] = v.(string)
	}

	// The setup token goes in a header rather than the body, so this call does
	// not go through doJSON.
	headers := map[string]string{}
	if v, ok := d.GetOk("setup_token"); ok {
		headers["X-Setup-Token"] = v.(string)
	}
	if err := doJSONWithHeaders(ctx, client, http.MethodPost, client.Endpoint+"/restore", payload, nil, headers); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore the Portainer backup: %w", err))
	}

	d.SetId("portainer-restore-" + fmt.Sprint(makeTimestamp()))
	return nil
}
