package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// azureAuthMethods are the ways Portainer can authenticate against Azure Blob
// Storage, as defined by portaineree.AzureAuthMethod.
var azureAuthMethods = []string{
	"servicePrincipalSecret",
	"servicePrincipalCertificate",
	"managedIdentity",
	"storageAccountKey",
}

// azureBlobBackupPayload turns the Azure fields into the JSON object Portainer
// expects. Empty optional fields are omitted so a payload never clears a
// credential that was configured elsewhere.
func azureBlobBackupPayload(d *schema.ResourceData, includeCron, writeOnly bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"authMethod":         d.Get("auth_method").(string),
		"storageAccountName": d.Get("storage_account_name").(string),
		"containerName":      d.Get("container_name").(string),
	}
	for field, key := range map[string]string{
		"service_url":                 "serviceUrl",
		"storage_account_key":         "storageAccountKey",
		"tenant_id":                   "tenantId",
		"client_id":                   "clientId",
		"client_secret":               "clientSecret",
		"client_certificate":          "clientCertificate",
		"client_certificate_password": "clientCertificatePassword",
		"managed_identity_client_id":  "managedIdentityClientId",
		"password":                    "password",
	} {
		v, _ := d.Get(field).(string)
		if writeOnly && azureBackupSecrets[field] {
			secret, err := twinSecret(d, field)
			if err != nil {
				return nil, err
			}
			v = secret
		}
		if v != "" {
			payload[key] = v
		}
	}
	if includeCron {
		if v, ok := d.GetOk("cron_rule"); ok && v.(string) != "" {
			payload["cronRule"] = v.(string)
		}
	}
	return payload, nil
}

// azureBackupSecrets are the Azure fields with a write-only twin.
var azureBackupSecrets = map[string]bool{
	"storage_account_key":         true,
	"client_secret":               true,
	"client_certificate":          true,
	"client_certificate_password": true,
	"password":                    true,
}

func resourceBackupAzureSettings() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupAzureSettingsWrite,
		ReadContext:   resourceBackupAzureSettingsRead,
		UpdateContext: resourceBackupAzureSettingsWrite,
		// Portainer has no endpoint to clear the settings; removing the resource
		// stops managing them and leaves the schedule as configured.
		DeleteContext: removeFromStateContext,
		Importer: &schema.ResourceImporter{
			StateContext: importSingleton("portainer-backup-azure-settings"),
		},

		// The Azure fields are spelled out in each of the three resources that
		// take them rather than shared through a helper, so docsdriftlint can
		// read the schemas statically and keep checking their pages for drift.
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
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				Description:   "Storage account access key. Used with `auth_method = \"storageAccountKey\"`. Stored in state as a sensitive value; use `storage_account_key_wo` to keep it out of state.",
				ConflictsWith: []string{"storage_account_key_wo"},
			},
			"storage_account_key_wo":         writeOnlySecretSchema("storage_account_key", "storage_account_key_wo_version", "Azure storage account key"),
			"storage_account_key_wo_version": writeOnlyVersionSchema("storage_account_key_wo", false),
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
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				Description:   "Client secret of the service principal. Used with `auth_method = \"servicePrincipalSecret\"`. Stored in state as a sensitive value; use `client_secret_wo` to keep it out of state.",
				ConflictsWith: []string{"client_secret_wo"},
			},
			"client_secret_wo":         writeOnlySecretSchema("client_secret", "client_secret_wo_version", "Azure client secret"),
			"client_secret_wo_version": writeOnlyVersionSchema("client_secret_wo", false),
			"client_certificate": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				Description:   "PEM-encoded client certificate of the service principal. Used with `auth_method = \"servicePrincipalCertificate\"`. Stored in state as a sensitive value; use `client_certificate_wo` to keep it out of state.",
				ConflictsWith: []string{"client_certificate_wo"},
			},
			"client_certificate_wo":         writeOnlySecretSchema("client_certificate", "client_certificate_wo_version", "Azure client certificate"),
			"client_certificate_wo_version": writeOnlyVersionSchema("client_certificate_wo", false),
			"client_certificate_password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				Description:   "Password protecting `client_certificate`, when it is encrypted. Stored in state as a sensitive value; use `client_certificate_password_wo` to keep it out of state.",
				ConflictsWith: []string{"client_certificate_password_wo"},
			},
			"client_certificate_password_wo":         writeOnlySecretSchema("client_certificate_password", "client_certificate_password_wo_version", "password of the Azure client certificate"),
			"client_certificate_password_wo_version": writeOnlyVersionSchema("client_certificate_password_wo", false),
			"managed_identity_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Client ID of the user-assigned managed identity. Used with `auth_method = \"managedIdentity\"`; leave unset for a system-assigned identity.",
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				Description:   "Password the backup archive itself is encrypted with. Leave unset for an unencrypted archive. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
				ConflictsWith: []string{"password_wo"},
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password used to encrypt the backup archive"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", false),
			"cron_rule": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Cron expression for the scheduled backup (for example `0 2 * * *`). Leave unset to keep the schedule off and back up only on demand.",
			},
			// Computed attributes
			"last_run_failed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the most recent scheduled Azure backup failed.",
			},
			"last_run_timestamp": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "UTC timestamp of the most recent scheduled Azure backup, empty when none has run.",
			},
		},
	}
}

func resourceBackupAzureSettingsWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload, err := azureBlobBackupPayload(d, true, true)
	if err != nil {
		return diag.FromErr(err)
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/azure/settings", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the Azure Blob backup settings: %w", err))
	}

	d.SetId("portainer-backup-azure-settings")
	return resourceBackupAzureSettingsRead(ctx, d, meta)
}

func resourceBackupAzureSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var settings struct {
		AuthMethod              string `json:"authMethod"`
		StorageAccountName      string `json:"storageAccountName"`
		ContainerName           string `json:"containerName"`
		ServiceURL              string `json:"serviceUrl"`
		TenantID                string `json:"tenantId"`
		ClientID                string `json:"clientId"`
		ManagedIdentityClientID string `json:"managedIdentityClientId"`
		CronRule                string `json:"cronRule"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/azure/settings", nil, &settings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Azure Blob backup settings: %w", err))
	}

	// Secrets are deliberately not read back: Portainer returns them, and
	// overwriting the configured values with whatever the server holds would
	// both leak them into state from elsewhere and fight the configuration.
	fields := map[string]interface{}{
		"auth_method":                settings.AuthMethod,
		"storage_account_name":       settings.StorageAccountName,
		"container_name":             settings.ContainerName,
		"service_url":                settings.ServiceURL,
		"tenant_id":                  settings.TenantID,
		"client_id":                  settings.ClientID,
		"managed_identity_client_id": settings.ManagedIdentityClientID,
		"cron_rule":                  settings.CronRule,
	}

	// The status endpoint is informational; a instance that has never run a
	// scheduled backup should not fail the read.
	var status struct {
		Failed       bool   `json:"Failed"`
		TimestampUTC string `json:"TimestampUTC"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/azure/status", nil, &status); err == nil {
		fields["last_run_failed"] = status.Failed
		fields["last_run_timestamp"] = status.TimestampUTC
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
