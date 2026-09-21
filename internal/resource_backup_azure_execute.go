package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceBackupAzureExecute() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupAzureExecuteCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"auth_method": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice(azureAuthMethods, false),
				Description:  "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`. The other fields required depend on which one is chosen.",
			},
			"storage_account_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the Azure storage account holding the container.",
			},
			"container_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Blob container the backups are written to.",
			},
			"service_url": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Blob service endpoint, for a sovereign cloud or a storage emulator. Leave unset for the public Azure cloud.",
			},
			"storage_account_key": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Storage account access key. Used with `auth_method = \"storageAccountKey\"`.",
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Entra ID tenant of the service principal. Used with both `servicePrincipal*` methods.",
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Application (client) ID of the service principal.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Client secret of the service principal. Used with `auth_method = \"servicePrincipalSecret\"`.",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "PEM-encoded client certificate of the service principal. Used with `auth_method = \"servicePrincipalCertificate\"`.",
			},
			"client_certificate_password": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Password protecting `client_certificate`, when it is encrypted.",
			},
			"managed_identity_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Client ID of the user-assigned managed identity. Used with `auth_method = \"managedIdentity\"`; leave unset for a system-assigned identity.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Password the backup archive itself is encrypted with. Leave unset for an unencrypted archive.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another backup run when they change. A backup is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceBackupAzureExecuteCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// The ad-hoc endpoint takes the same settings object as the scheduled one
	// but without a cron rule: it backs up once with the credentials given and
	// does not change the stored schedule.
	payload := azureBlobBackupPayload(d, false)
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/azure/execute", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to execute the Azure Blob backup: %w", err))
	}

	d.SetId(fmt.Sprintf("portainer-backup-azure-execute-%d", makeTimestamp()))
	return nil
}
