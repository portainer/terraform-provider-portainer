package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*backupAzureConnectionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*backupAzureConnectionDataSource)(nil)
)

func newBackupAzureConnectionDataSource() datasource.DataSource {
	return &backupAzureConnectionDataSource{}
}

type backupAzureConnectionDataSource struct{ client *APIClient }

type backupAzureConnectionModel struct {
	ID types.String `tfsdk:"id"`
	azureBlobModel
	FailOnError types.Bool   `tfsdk:"fail_on_error"`
	Success     types.Bool   `tfsdk:"success"`
	Error       types.String `tfsdk:"error"`
}

func (d *backupAzureConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_azure_connection"
}

func (d *backupAzureConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	opt := func(desc string, sensitive bool) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Sensitive: sensitive, Description: desc}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"auth_method": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf(azureAuthMethods...)},
				Description: "How Portainer authenticates against Azure: `servicePrincipalSecret`, `servicePrincipalCertificate`, `managedIdentity` or `storageAccountKey`. The other fields required depend on which one is chosen.",
			},
			"storage_account_name":        schema.StringAttribute{Required: true, Description: "Name of the Azure storage account holding the container."},
			"container_name":              schema.StringAttribute{Required: true, Description: "Blob container the backups are written to."},
			"service_url":                 opt("Blob service endpoint, for a sovereign cloud or a storage emulator. Leave unset for the public Azure cloud.", false),
			"storage_account_key":         opt("Storage account access key. Used with `auth_method = \"storageAccountKey\"`.", true),
			"tenant_id":                   opt("Entra ID tenant of the service principal. Used with both `servicePrincipal*` methods.", false),
			"client_id":                   opt("Application (client) ID of the service principal.", false),
			"client_secret":               opt("Client secret of the service principal. Used with `auth_method = \"servicePrincipalSecret\"`.", true),
			"client_certificate":          opt("PEM-encoded client certificate of the service principal. Used with `auth_method = \"servicePrincipalCertificate\"`.", true),
			"client_certificate_password": opt("Password protecting `client_certificate`, when it is encrypted.", true),
			"managed_identity_client_id":  opt("Client ID of the user-assigned managed identity. Used with `auth_method = \"managedIdentity\"`; leave unset for a system-assigned identity.", false),
			"password":                    opt("Password the backup archive itself is encrypted with. Leave unset for an unencrypted archive.", true),
			// Data sources take no defaults: a null value reads as true.
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether an unreachable container makes the data source itself fail. Defaults to true, which is the point of a pre-flight check; set it to false to branch on `success` instead.",
			},
			"success": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer could reach the Azure Blob container with these credentials.",
			},
			"error": schema.StringAttribute{
				Computed:    true,
				Description: "Reason the check failed, empty on success.",
			},
		},
	}
}

func (d *backupAzureConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *backupAzureConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m backupAzureConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.FailOnError.IsNull() {
		m.FailOnError = types.BoolValue(true)
	}

	payload := azureBlobPayload(ctx, &m.azureBlobModel, nil, nil, azureBlobSettingsRequiredKeys, azureBlobSettingsKeys, &resp.Diagnostics)

	// The endpoint answers with a status code rather than a result object, so
	// the outcome is the code itself.
	err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/backup/azure/test", payload, nil)
	success := err == nil
	message := ""
	if err != nil {
		var se *apiStatusError
		if !errors.As(err, &se) {
			// A transport failure is not an answer from Azure, so it is raised
			// rather than reported as a failed check.
			resp.Diagnostics.AddError("Failed to test the Azure Blob backup connection", fmt.Sprintf("failed to test the Azure Blob backup connection: %s", err))
			return
		}
		message = se.Body
	}

	m.Success = types.BoolValue(success)
	m.Error = types.StringValue(message)
	m.ID = types.StringValue("azure-backup-test-" + m.StorageAccountName.ValueString() + "/" + m.ContainerName.ValueString())

	if !success && m.FailOnError.ValueBool() {
		resp.Diagnostics.AddError("Azure Blob backup connection test failed", fmt.Sprintf("the Azure Blob backup connection test failed: %s", message))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
