package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// azureAuthMethods are the ways Portainer can authenticate against Azure Blob
// Storage, as defined by portaineree.AzureAuthMethod.
var azureAuthMethods = []string{
	"servicePrincipalSecret",
	"servicePrincipalCertificate",
	"managedIdentity",
	"storageAccountKey",
}

// azureBackupSecrets are the Azure fields with a write-only twin.
var azureBackupSecrets = map[string]bool{
	"storage_account_key":         true,
	"client_secret":               true,
	"client_certificate":          true,
	"client_certificate_password": true,
	"password":                    true,
}

// azureBlobFieldOrder lists the optional Azure fields in payload order.
var azureBlobFieldOrder = []string{
	"service_url", "storage_account_key", "tenant_id", "client_id", "client_secret",
	"client_certificate", "client_certificate_password", "managed_identity_client_id", "password",
}

// azureBlobSettingsKeys are the JSON keys of the backup settings payloads
// (camelCase).
var azureBlobSettingsKeys = map[string]string{
	"service_url":                 "serviceUrl",
	"storage_account_key":         "storageAccountKey",
	"tenant_id":                   "tenantId",
	"client_id":                   "clientId",
	"client_secret":               "clientSecret",
	"client_certificate":          "clientCertificate",
	"client_certificate_password": "clientCertificatePassword",
	"managed_identity_client_id":  "managedIdentityClientId",
	"password":                    "password",
}

// azureBlobModel holds the Azure fields every Azure backup type takes.
type azureBlobModel struct {
	AuthMethod                types.String `tfsdk:"auth_method"`
	StorageAccountName        types.String `tfsdk:"storage_account_name"`
	ContainerName             types.String `tfsdk:"container_name"`
	ServiceURL                types.String `tfsdk:"service_url"`
	StorageAccountKey         types.String `tfsdk:"storage_account_key"`
	TenantID                  types.String `tfsdk:"tenant_id"`
	ClientID                  types.String `tfsdk:"client_id"`
	ClientSecret              types.String `tfsdk:"client_secret"`
	ClientCertificate         types.String `tfsdk:"client_certificate"`
	ClientCertificatePassword types.String `tfsdk:"client_certificate_password"`
	ManagedIdentityClientID   types.String `tfsdk:"managed_identity_client_id"`
	Password                  types.String `tfsdk:"password"`
}

// azureBlobWriteOnlyModel holds the write-only twins of the Azure secrets.
type azureBlobWriteOnlyModel struct {
	StorageAccountKeyWO                types.String `tfsdk:"storage_account_key_wo"`
	StorageAccountKeyWOVersion         types.Int64  `tfsdk:"storage_account_key_wo_version"`
	ClientSecretWO                     types.String `tfsdk:"client_secret_wo"`
	ClientSecretWOVersion              types.Int64  `tfsdk:"client_secret_wo_version"`
	ClientCertificateWO                types.String `tfsdk:"client_certificate_wo"`
	ClientCertificateWOVersion         types.Int64  `tfsdk:"client_certificate_wo_version"`
	ClientCertificatePasswordWO        types.String `tfsdk:"client_certificate_password_wo"`
	ClientCertificatePasswordWOVersion types.Int64  `tfsdk:"client_certificate_password_wo_version"`
	PasswordWO                         types.String `tfsdk:"password_wo"`
	PasswordWOVersion                  types.Int64  `tfsdk:"password_wo_version"`
}

func (m *azureBlobModel) field(name string) types.String {
	switch name {
	case "service_url":
		return m.ServiceURL
	case "storage_account_key":
		return m.StorageAccountKey
	case "tenant_id":
		return m.TenantID
	case "client_id":
		return m.ClientID
	case "client_secret":
		return m.ClientSecret
	case "client_certificate":
		return m.ClientCertificate
	case "client_certificate_password":
		return m.ClientCertificatePassword
	case "managed_identity_client_id":
		return m.ManagedIdentityClientID
	case "password":
		return m.Password
	}
	return types.StringNull()
}

func (w *azureBlobWriteOnlyModel) version(name string) types.Int64 {
	switch name {
	case "storage_account_key":
		return w.StorageAccountKeyWOVersion
	case "client_secret":
		return w.ClientSecretWOVersion
	case "client_certificate":
		return w.ClientCertificateWOVersion
	case "client_certificate_password":
		return w.ClientCertificatePasswordWOVersion
	case "password":
		return w.PasswordWOVersion
	}
	return types.Int64Null()
}

// clearWriteOnly nulls the write-only values, which are never stored.
func (w *azureBlobWriteOnlyModel) clearWriteOnly() {
	w.StorageAccountKeyWO = types.StringNull()
	w.ClientSecretWO = types.StringNull()
	w.ClientCertificateWO = types.StringNull()
	w.ClientCertificatePasswordWO = types.StringNull()
	w.PasswordWO = types.StringNull()
}

// azureBlobPayload turns the Azure fields into the JSON object Portainer
// expects, under the given keys. Empty optional fields are omitted so a
// payload never clears a credential that was configured elsewhere. With wo
// set, a secret's write-only twin from config is used when configured.
func azureBlobPayload(ctx context.Context, m *azureBlobModel, wo *azureBlobWriteOnlyModel, config *tfsdk.Config, required, keys map[string]string, diags *diag.Diagnostics) map[string]interface{} {
	payload := map[string]interface{}{
		required["auth_method"]:          m.AuthMethod.ValueString(),
		required["storage_account_name"]: m.StorageAccountName.ValueString(),
		required["container_name"]:       m.ContainerName.ValueString(),
	}
	for _, field := range azureBlobFieldOrder {
		v := m.field(field).ValueString()
		if wo != nil && config != nil && azureBackupSecrets[field] {
			v, _ = gsbTwinSecret(ctx, *config, m.field(field), path.Root(field+"_wo"), wo.version(field), diags)
		}
		if v != "" {
			payload[keys[field]] = v
		}
	}
	return payload
}

var azureBlobSettingsRequiredKeys = map[string]string{
	"auth_method": "authMethod", "storage_account_name": "storageAccountName", "container_name": "containerName",
}

// azureBlobDescriptions are the descriptions of the Azure fields shared by
// the settings, execute and connection-test types.
var azureBlobDescriptions = map[string]string{
	"auth_method":                 "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`. The other fields required depend on which one is chosen.",
	"storage_account_name":        "Name of the Azure storage account holding the container.",
	"container_name":              "Blob container the backups are written to.",
	"service_url":                 "Blob service endpoint, for a sovereign cloud or a storage emulator. Leave unset for the public Azure cloud.",
	"storage_account_key":         "Storage account access key. Used with `auth_method = \"storageAccountKey\"`. Stored in state as a sensitive value; use `storage_account_key_wo` to keep it out of state.",
	"tenant_id":                   "Entra ID tenant of the service principal. Used with both `servicePrincipal*` methods.",
	"client_id":                   "Application (client) ID of the service principal.",
	"client_secret":               "Client secret of the service principal. Used with `auth_method = \"servicePrincipalSecret\"`. Stored in state as a sensitive value; use `client_secret_wo` to keep it out of state.",
	"client_certificate":          "PEM-encoded client certificate of the service principal. Used with `auth_method = \"servicePrincipalCertificate\"`. Stored in state as a sensitive value; use `client_certificate_wo` to keep it out of state.",
	"client_certificate_password": "Password protecting `client_certificate`, when it is encrypted. Stored in state as a sensitive value; use `client_certificate_password_wo` to keep it out of state.",
	"managed_identity_client_id":  "Client ID of the user-assigned managed identity. Used with `auth_method = \"managedIdentity\"`; leave unset for a system-assigned identity.",
	"password":                    "Password the backup archive itself is encrypted with. Leave unset for an unencrypted archive. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
}

// azureBlobWriteOnlyWhat names each secret in its write-only twin's
// description.
var azureBlobWriteOnlyWhat = map[string]string{ //nolint:gosec // descriptions, not credentials
	"storage_account_key":         "Azure storage account key",
	"client_secret":               "Azure client secret",
	"client_certificate":          "Azure client certificate",
	"client_certificate_password": "password of the Azure client certificate",
	"password":                    "password used to encrypt the backup archive",
}

// azureBlobAttributes is the schema of the Azure fields of a resource, with
// the write-only twins of the secrets. forceNew replaces the resource on any
// change; required and optional fields are described by desc.
func azureBlobAttributes(desc, woWhat map[string]string, forceNew bool) map[string]schema.Attribute {
	var replace []planmodifier.String
	if forceNew {
		replace = []planmodifier.String{stringplanmodifier.RequiresReplace()}
	}
	attrs := map[string]schema.Attribute{
		"auth_method": schema.StringAttribute{
			Required:      true,
			Validators:    []validator.String{stringvalidator.OneOf(azureAuthMethods...)},
			PlanModifiers: replace,
			Description:   desc["auth_method"],
		},
		"storage_account_name": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: desc["storage_account_name"]},
		"container_name":       schema.StringAttribute{Required: true, PlanModifiers: replace, Description: desc["container_name"]},
	}
	for _, field := range azureBlobFieldOrder {
		a := schema.StringAttribute{Optional: true, Description: desc[field]}
		if forceNew {
			a.PlanModifiers = []planmodifier.String{gsbRequiresReplaceString()}
		}
		if azureBackupSecrets[field] {
			a.Sensitive = true
			attrs[field+"_wo"] = gsbWriteOnlySecretAttribute(field, field+"_wo_version", woWhat[field])
			attrs[field+"_wo_version"] = gsbWriteOnlyVersionAttribute(field+"_wo", forceNew)
		}
		attrs[field] = a
	}
	return attrs
}

var (
	_ resource.Resource                = (*backupAzureSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*backupAzureSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*backupAzureSettingsResource)(nil)
)

const backupAzureSettingsID = "portainer-backup-azure-settings"

func newBackupAzureSettingsResource() resource.Resource { return &backupAzureSettingsResource{} }

type backupAzureSettingsResource struct{ client *APIClient }

type backupAzureSettingsModel struct {
	ID types.String `tfsdk:"id"`
	azureBlobModel
	azureBlobWriteOnlyModel
	CronRule         types.String `tfsdk:"cron_rule"`
	LastRunFailed    types.Bool   `tfsdk:"last_run_failed"`
	LastRunTimestamp types.String `tfsdk:"last_run_timestamp"`
}

func (r *backupAzureSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_azure_settings"
}

func (r *backupAzureSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := azureBlobAttributes(azureBlobDescriptions, azureBlobWriteOnlyWhat, false)
	attrs["id"] = schema.StringAttribute{
		Description:   "Identifier of the resource.",
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
	attrs["cron_rule"] = schema.StringAttribute{
		Optional:    true,
		Description: "Cron expression for the scheduled backup (for example `0 2 * * *`). Leave unset to keep the schedule off and back up only on demand.",
	}
	attrs["last_run_failed"] = schema.BoolAttribute{
		Computed:    true,
		Description: "Whether the most recent scheduled Azure backup failed.",
	}
	attrs["last_run_timestamp"] = schema.StringAttribute{
		Computed:    true,
		Description: "UTC timestamp of the most recent scheduled Azure backup, empty when none has run.",
	}
	resp.Schema = schema.Schema{Attributes: attrs}
}

func (r *backupAzureSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupAzureSettingsResource) write(ctx context.Context, config tfsdk.Config, plan *backupAzureSettingsModel, diags *diag.Diagnostics) {
	payload := azureBlobPayload(ctx, &plan.azureBlobModel, &plan.azureBlobWriteOnlyModel, &config, azureBlobSettingsRequiredKeys, azureBlobSettingsKeys, diags)
	if diags.HasError() {
		return
	}
	if v := plan.CronRule.ValueString(); v != "" {
		payload["cronRule"] = v
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/backup/azure/settings", payload, nil); err != nil {
		diags.AddError("Failed to update the Azure Blob backup settings", fmt.Sprintf("failed to update the Azure Blob backup settings: %s", err))
		return
	}
	plan.ID = types.StringValue(backupAzureSettingsID)
	plan.clearWriteOnly()

	// The configured values are what was just written; only the computed
	// status comes from the read (the framework rejects a configured value
	// changing during apply, and a difference shows on the next refresh).
	read := *plan
	read.LastRunFailed = types.BoolNull()
	read.LastRunTimestamp = types.StringNull()
	r.read(ctx, &read, true, diags)
	plan.LastRunFailed = read.LastRunFailed
	plan.LastRunTimestamp = read.LastRunTimestamp
}

func (r *backupAzureSettingsResource) read(ctx context.Context, m *backupAzureSettingsModel, fw bool, diags *diag.Diagnostics) {
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
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/azure/settings", nil, &settings); err != nil {
		diags.AddError("Failed to read the Azure Blob backup settings", fmt.Sprintf("failed to read the Azure Blob backup settings: %s", err))
		return
	}

	// Secrets are deliberately not read back: Portainer returns them, and
	// overwriting the configured values with whatever the server holds would
	// both leak them into state from elsewhere and fight the configuration.
	m.AuthMethod = types.StringValue(settings.AuthMethod)
	m.StorageAccountName = types.StringValue(settings.StorageAccountName)
	m.ContainerName = types.StringValue(settings.ContainerName)
	m.ServiceURL = gsbString(settings.ServiceURL, m.ServiceURL, fw)
	m.TenantID = gsbString(settings.TenantID, m.TenantID, fw)
	m.ClientID = gsbString(settings.ClientID, m.ClientID, fw)
	m.ManagedIdentityClientID = gsbString(settings.ManagedIdentityClientID, m.ManagedIdentityClientID, fw)
	m.CronRule = gsbString(settings.CronRule, m.CronRule, fw)

	// The status endpoint is informational; a instance that has never run a
	// scheduled backup should not fail the read.
	var status struct {
		Failed       bool   `json:"Failed"`
		TimestampUTC string `json:"TimestampUTC"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/azure/status", nil, &status); err == nil {
		m.LastRunFailed = types.BoolValue(status.Failed)
		m.LastRunTimestamp = types.StringValue(status.TimestampUTC)
	}
}

func (r *backupAzureSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupAzureSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *backupAzureSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state backupAzureSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &state, gsbFrameworkState(ctx, req.Private), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *backupAzureSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan backupAzureSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the settings: Portainer has no endpoint to clear them,
// so removing the resource leaves the schedule as configured.
func (r *backupAzureSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts any ID: there is one set of settings per instance.
func (r *backupAzureSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), backupAzureSettingsID)...)
}
