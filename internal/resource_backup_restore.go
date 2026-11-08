package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Portainer's restore payloads use PascalCase field names, unlike the backup
// settings payloads next to them, which are camelCase. Getting this wrong is
// silent: the request is accepted and the fields simply arrive empty.

func resourceBackupS3Restore() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupS3RestoreCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"bucket_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "S3 bucket holding the backup archive.",
			},
			"filename": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the archive in the bucket to restore from.",
			},
			"region": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "AWS region of the bucket.",
			},
			"access_key_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Access key ID used to read the bucket. Stored in state as a sensitive value.",
			},
			"secret_access_key": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"secret_access_key", "secret_access_key_wo"},
				ForceNew:     true,
				Sensitive:    true,
				Description:  "Secret access key paired with `access_key_id`. Stored in state as a sensitive value; use `secret_access_key_wo` to keep it out of state.",
			},
			"secret_access_key_wo":         writeOnlySecretSchema("secret_access_key", "secret_access_key_wo_version", "S3 secret access key"),
			"secret_access_key_wo_version": writeOnlyVersionSchema("secret_access_key_wo", true),
			"s3_compatible_host": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Endpoint of an S3-compatible service such as MinIO. Leave unset for AWS S3.",
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Password the archive was encrypted with. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
				ConflictsWith: []string{"password_wo"},
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password the backup archive was encrypted with"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", true),
		},
	}
}

func resourceBackupS3RestoreCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	secretAccessKey, err := twinSecret(d, "secret_access_key")
	if err != nil {
		return diag.FromErr(err)
	}
	payload := map[string]interface{}{
		"BucketName":      d.Get("bucket_name").(string),
		"Filename":        d.Get("filename").(string),
		"Region":          d.Get("region").(string),
		"AccessKeyID":     d.Get("access_key_id").(string),
		"SecretAccessKey": secretAccessKey,
	}
	if v, ok := d.GetOk("s3_compatible_host"); ok && v.(string) != "" {
		payload["S3CompatibleHost"] = v.(string)
	}
	password, err := twinSecret(d, "password")
	if err != nil {
		return diag.FromErr(err)
	}
	if password != "" {
		payload["Password"] = password
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/s3/restore", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore from the S3 backup %q: %w", d.Get("filename").(string), err))
	}

	d.SetId(fmt.Sprintf("portainer-restore-s3-%d", makeTimestamp()))
	return nil
}

func resourceBackupAzureRestore() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupAzureRestoreCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"auth_method": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice(azureAuthMethods, false),
				Description:  "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`.",
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
				Description: "Blob container holding the backup archive.",
			},
			"blob_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the blob to restore from.",
			},
			"service_url": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Blob service endpoint, for a sovereign cloud or a storage emulator.",
			},
			"storage_account_key": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Storage account access key. Used with `auth_method = \"storageAccountKey\"`. Stored in state as a sensitive value; use `storage_account_key_wo` to keep it out of state.",
				ConflictsWith: []string{"storage_account_key_wo"},
			},
			"storage_account_key_wo":         writeOnlySecretSchema("storage_account_key", "storage_account_key_wo_version", "Azure storage account key"),
			"storage_account_key_wo_version": writeOnlyVersionSchema("storage_account_key_wo", true),
			"tenant_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Entra ID tenant of the service principal.",
			},
			"client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Application (client) ID of the service principal.",
			},
			"client_secret": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Client secret of the service principal. Stored in state as a sensitive value; use `client_secret_wo` to keep it out of state.",
				ConflictsWith: []string{"client_secret_wo"},
			},
			"client_secret_wo":         writeOnlySecretSchema("client_secret", "client_secret_wo_version", "Azure client secret"),
			"client_secret_wo_version": writeOnlyVersionSchema("client_secret_wo", true),
			"client_certificate": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "PEM-encoded client certificate of the service principal. Stored in state as a sensitive value; use `client_certificate_wo` to keep it out of state.",
				ConflictsWith: []string{"client_certificate_wo"},
			},
			"client_certificate_wo":         writeOnlySecretSchema("client_certificate", "client_certificate_wo_version", "Azure client certificate"),
			"client_certificate_wo_version": writeOnlyVersionSchema("client_certificate_wo", true),
			"client_certificate_password": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Password protecting `client_certificate`. Stored in state as a sensitive value; use `client_certificate_password_wo` to keep it out of state.",
				ConflictsWith: []string{"client_certificate_password_wo"},
			},
			"client_certificate_password_wo":         writeOnlySecretSchema("client_certificate_password", "client_certificate_password_wo_version", "password of the Azure client certificate"),
			"client_certificate_password_wo_version": writeOnlyVersionSchema("client_certificate_password_wo", true),
			"managed_identity_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Client ID of the user-assigned managed identity.",
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				Description:   "Password the archive was encrypted with. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
				ConflictsWith: []string{"password_wo"},
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password the backup archive was encrypted with"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", true),
		},
	}
}

func resourceBackupAzureRestoreCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"AuthMethod":         d.Get("auth_method").(string),
		"StorageAccountName": d.Get("storage_account_name").(string),
		"ContainerName":      d.Get("container_name").(string),
		"BlobName":           d.Get("blob_name").(string),
	}
	for field, key := range map[string]string{
		"service_url":                 "ServiceURL",
		"storage_account_key":         "StorageAccountKey",
		"tenant_id":                   "TenantID",
		"client_id":                   "ClientID",
		"client_secret":               "ClientSecret",
		"client_certificate":          "ClientCertificate",
		"client_certificate_password": "ClientCertificatePassword",
		"managed_identity_client_id":  "ManagedIdentityClientID",
		"password":                    "Password",
	} {
		v := d.Get(field).(string)
		if azureBackupSecrets[field] {
			secret, err := twinSecret(d, field)
			if err != nil {
				return diag.FromErr(err)
			}
			v = secret
		}
		if v != "" {
			payload[key] = v
		}
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/azure/restore", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore from the Azure Blob backup %q: %w", d.Get("blob_name").(string), err))
	}

	d.SetId(fmt.Sprintf("portainer-restore-azure-%d", makeTimestamp()))
	return nil
}
