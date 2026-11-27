package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Portainer's restore payloads use PascalCase field names, unlike the backup
// settings payloads next to them, which are camelCase. Getting this wrong is
// silent: the request is accepted and the fields simply arrive empty.

var (
	_ resource.Resource                     = (*backupS3RestoreResource)(nil)
	_ resource.ResourceWithConfigure        = (*backupS3RestoreResource)(nil)
	_ resource.ResourceWithConfigValidators = (*backupS3RestoreResource)(nil)
	_ resource.Resource                     = (*backupAzureRestoreResource)(nil)
	_ resource.ResourceWithConfigure        = (*backupAzureRestoreResource)(nil)
)

func newBackupS3RestoreResource() resource.Resource { return &backupS3RestoreResource{} }

type backupS3RestoreResource struct{ client *APIClient }

type backupS3RestoreModel struct {
	ID                       types.String `tfsdk:"id"`
	BucketName               types.String `tfsdk:"bucket_name"`
	Filename                 types.String `tfsdk:"filename"`
	Region                   types.String `tfsdk:"region"`
	AccessKeyID              types.String `tfsdk:"access_key_id"`
	SecretAccessKey          types.String `tfsdk:"secret_access_key"`
	SecretAccessKeyWO        types.String `tfsdk:"secret_access_key_wo"`
	SecretAccessKeyWOVersion types.Int64  `tfsdk:"secret_access_key_wo_version"`
	S3CompatibleHost         types.String `tfsdk:"s3_compatible_host"`
	Password                 types.String `tfsdk:"password"`
	PasswordWO               types.String `tfsdk:"password_wo"`
	PasswordWOVersion        types.Int64  `tfsdk:"password_wo_version"`
}

func (r *backupS3RestoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_s3_restore"
}

func (r *backupS3RestoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bucket_name": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "S3 bucket holding the backup archive."},
			"filename":    schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Name of the archive in the bucket to restore from."},
			"region":      schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "AWS region of the bucket."},
			"access_key_id": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Access key ID used to read the bucket. Stored in state as a sensitive value.",
			},
			"secret_access_key": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Secret access key paired with `access_key_id`. Stored in state as a sensitive value; use `secret_access_key_wo` to keep it out of state.",
			},
			"secret_access_key_wo":         gsbWriteOnlySecretAttribute("secret_access_key", "secret_access_key_wo_version", "S3 secret access key"),
			"secret_access_key_wo_version": gsbWriteOnlyVersionAttribute("secret_access_key_wo", true),
			"s3_compatible_host": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: replace,
				Description:   "Endpoint of an S3-compatible service such as MinIO. Leave unset for AWS S3.",
			},
			"password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Password the archive was encrypted with. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password the backup archive was encrypted with"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", true),
		},
	}
}

func (r *backupS3RestoreResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("secret_access_key"), path.MatchRoot("secret_access_key_wo")),
	}
}

func (r *backupS3RestoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupS3RestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupS3RestoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secretAccessKey, _ := gsbTwinSecret(ctx, req.Config, plan.SecretAccessKey, path.Root("secret_access_key_wo"), plan.SecretAccessKeyWOVersion, &resp.Diagnostics)
	password, _ := gsbTwinSecret(ctx, req.Config, plan.Password, path.Root("password_wo"), plan.PasswordWOVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"BucketName":      plan.BucketName.ValueString(),
		"Filename":        plan.Filename.ValueString(),
		"Region":          plan.Region.ValueString(),
		"AccessKeyID":     plan.AccessKeyID.ValueString(),
		"SecretAccessKey": secretAccessKey,
	}
	if v := plan.S3CompatibleHost.ValueString(); v != "" {
		payload["S3CompatibleHost"] = v
	}
	if password != "" {
		payload["Password"] = password
	}

	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/backup/s3/restore", payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to restore from the S3 backup", fmt.Sprintf("failed to restore from the S3 backup %q: %s", plan.Filename.ValueString(), err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("portainer-restore-s3-%d", makeTimestamp()))
	plan.SecretAccessKeyWO = types.StringNull()
	plan.PasswordWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: a restore is a one-shot action.
func (r *backupS3RestoreResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *backupS3RestoreResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Restores cannot be updated", "Every attribute of portainer_backup_s3_restore forces a new resource.")
}

// Delete only forgets the restore, which cannot be undone.
func (r *backupS3RestoreResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func newBackupAzureRestoreResource() resource.Resource { return &backupAzureRestoreResource{} }

type backupAzureRestoreResource struct{ client *APIClient }

type backupAzureRestoreModel struct {
	ID types.String `tfsdk:"id"`
	azureBlobModel
	azureBlobWriteOnlyModel
	BlobName types.String `tfsdk:"blob_name"`
}

// azureBlobRestoreDescriptions are the descriptions of the Azure fields of
// portainer_backup_azure_restore.
var azureBlobRestoreDescriptions = map[string]string{
	"auth_method":                 "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`.",
	"storage_account_name":        "Name of the Azure storage account holding the container.",
	"container_name":              "Blob container holding the backup archive.",
	"service_url":                 "Blob service endpoint, for a sovereign cloud or a storage emulator.",
	"storage_account_key":         "Storage account access key. Used with `auth_method = \"storageAccountKey\"`. Stored in state as a sensitive value; use `storage_account_key_wo` to keep it out of state.",
	"tenant_id":                   "Entra ID tenant of the service principal.",
	"client_id":                   "Application (client) ID of the service principal.",
	"client_secret":               "Client secret of the service principal. Stored in state as a sensitive value; use `client_secret_wo` to keep it out of state.",
	"client_certificate":          "PEM-encoded client certificate of the service principal. Stored in state as a sensitive value; use `client_certificate_wo` to keep it out of state.",
	"client_certificate_password": "Password protecting `client_certificate`. Stored in state as a sensitive value; use `client_certificate_password_wo` to keep it out of state.",
	"managed_identity_client_id":  "Client ID of the user-assigned managed identity.",
	"password":                    "Password the archive was encrypted with. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
}

var azureBlobRestoreWriteOnlyWhat = map[string]string{ //nolint:gosec // descriptions, not credentials
	"storage_account_key":         "Azure storage account key",
	"client_secret":               "Azure client secret",
	"client_certificate":          "Azure client certificate",
	"client_certificate_password": "password of the Azure client certificate",
	"password":                    "password the backup archive was encrypted with",
}

var azureBlobRestoreRequiredKeys = map[string]string{
	"auth_method": "AuthMethod", "storage_account_name": "StorageAccountName", "container_name": "ContainerName",
}

var azureBlobRestoreKeys = map[string]string{
	"service_url":                 "ServiceURL",
	"storage_account_key":         "StorageAccountKey",
	"tenant_id":                   "TenantID",
	"client_id":                   "ClientID",
	"client_secret":               "ClientSecret",
	"client_certificate":          "ClientCertificate",
	"client_certificate_password": "ClientCertificatePassword",
	"managed_identity_client_id":  "ManagedIdentityClientID",
	"password":                    "Password",
}

func (r *backupAzureRestoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_azure_restore"
}

func (r *backupAzureRestoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := azureBlobAttributes(azureBlobRestoreDescriptions, azureBlobRestoreWriteOnlyWhat, true)
	// The restore never had empty and null values to tell apart (it is not
	// read back), so a plain RequiresReplace keeps SDKv2's ForceNew.
	for _, field := range azureBlobFieldOrder {
		a := attrs[field].(schema.StringAttribute)
		a.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
		attrs[field] = a
	}
	attrs["id"] = schema.StringAttribute{
		Description:   "Identifier of the resource.",
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
	attrs["blob_name"] = schema.StringAttribute{
		Required:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		Description:   "Name of the blob to restore from.",
	}
	resp.Schema = schema.Schema{Attributes: attrs}
}

func (r *backupAzureRestoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupAzureRestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupAzureRestoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := azureBlobPayload(ctx, &plan.azureBlobModel, &plan.azureBlobWriteOnlyModel, &req.Config, azureBlobRestoreRequiredKeys, azureBlobRestoreKeys, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	payload["BlobName"] = plan.BlobName.ValueString()

	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/backup/azure/restore", payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to restore from the Azure Blob backup", fmt.Sprintf("failed to restore from the Azure Blob backup %q: %s", plan.BlobName.ValueString(), err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("portainer-restore-azure-%d", makeTimestamp()))
	plan.clearWriteOnly()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: a restore is a one-shot action.
func (r *backupAzureRestoreResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *backupAzureRestoreResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Restores cannot be updated", "Every attribute of portainer_backup_azure_restore forces a new resource.")
}

// Delete only forgets the restore, which cannot be undone.
func (r *backupAzureRestoreResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
