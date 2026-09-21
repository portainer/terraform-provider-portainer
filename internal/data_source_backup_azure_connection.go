package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceBackupAzureConnection() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceBackupAzureConnectionRead,

		Schema: map[string]*schema.Schema{
			"auth_method": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice(azureAuthMethods, false),
				Description:  "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`. The other fields required depend on which one is chosen.",
			},
			"storage_account_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Azure storage account holding the container.",
			},
			"container_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Blob container the backups are written to.",
			},
			"service_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Blob service endpoint, for a sovereign cloud or a storage emulator. Leave unset for the public Azure cloud.",
			},
			"storage_account_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Storage account access key. Used with `auth_method = \"storageAccountKey\"`.",
			},
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Entra ID tenant of the service principal. Used with both `servicePrincipal*` methods.",
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Application (client) ID of the service principal.",
			},
			"client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Client secret of the service principal. Used with `auth_method = \"servicePrincipalSecret\"`.",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded client certificate of the service principal. Used with `auth_method = \"servicePrincipalCertificate\"`.",
			},
			"client_certificate_password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password protecting `client_certificate`, when it is encrypted.",
			},
			"managed_identity_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Client ID of the user-assigned managed identity. Used with `auth_method = \"managedIdentity\"`; leave unset for a system-assigned identity.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password the backup archive itself is encrypted with. Leave unset for an unencrypted archive.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether an unreachable container makes the data source itself fail. Defaults to true, which is the point of a pre-flight check; set it to false to branch on `success` instead.",
			},
			// Computed attributes
			"success": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach the Azure Blob container with these credentials.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Reason the check failed, empty on success.",
			},
		},
	}
}

func dataSourceBackupAzureConnectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := azureBlobBackupPayload(d, false)

	// The endpoint answers with a status code rather than a result object, so
	// the outcome is the code itself.
	err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/azure/test", payload, nil)
	success := err == nil
	message := ""
	if err != nil {
		var se *apiStatusError
		if errors.As(err, &se) {
			message = se.Body
		} else {
			// A transport failure is not an answer from Azure, so it is raised
			// rather than reported as a failed check.
			return diag.FromErr(fmt.Errorf("failed to test the Azure Blob backup connection: %w", err))
		}
	}

	if err := setFields(d, map[string]interface{}{"success": success, "error": message}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("azure-backup-test-" + d.Get("storage_account_name").(string) + "/" + d.Get("container_name").(string))

	if !success && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the Azure Blob backup connection test failed: %s", message))
	}
	return nil
}
