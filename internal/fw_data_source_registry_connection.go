package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = (*registryConnectionDataSource)(nil)

func newRegistryConnectionDataSource() datasource.DataSource { return &registryConnectionDataSource{} }

type registryConnectionDataSource struct{ client *APIClient }

type registryConnectionDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	URL         types.String `tfsdk:"url"`
	Type        types.Int64  `tfsdk:"type"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	TLS         types.Bool   `tfsdk:"tls"`
	FailOnError types.Bool   `tfsdk:"fail_on_error"`
	Success     types.Bool   `tfsdk:"success"`
	Message     types.String `tfsdk:"message"`
}

func (d *registryConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_connection"
}

func (d *registryConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the test: registry-ping-<url>."},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "URL of the registry to test, without a scheme for most types (for example `registry.example.com`).",
			},
			"type": schema.Int64Attribute{
				Required:    true,
				Validators:  []validator.Int64{int64validator.Between(1, 8)},
				Description: "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR, 8 = GitHub.",
			},
			// Portainer rejects a ping without credentials with 400 "Username and
			// password are required", for every registry type including DockerHub.
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username used for the test. Portainer requires credentials for every registry type, including public ones.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password or token paired with `username`.",
			},
			// Optional with a default of false in SDKv2: computed here so the
			// default still shows in the state.
			"tls": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the registry is contacted over TLS.",
			},
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether an unreachable registry makes the data source itself fail. Defaults to false, which reports the outcome in `success` and `message`.",
			},
			"success": schema.BoolAttribute{Computed: true, Description: "Whether Portainer could reach and authenticate against the registry."},
			"message": schema.StringAttribute{Computed: true, Description: "Message returned by Portainer describing the outcome, carrying the reason on failure."},
		},
	}
}

func (d *registryConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *registryConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data registryConnectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TLS = types.BoolValue(data.TLS.ValueBool())
	data.FailOnError = types.BoolValue(data.FailOnError.ValueBool())

	// registries.registryPingPayload uses PascalCase field names.
	payload := map[string]interface{}{
		"URL":      data.URL.ValueString(),
		"Type":     int(data.Type.ValueInt64()),
		"TLS":      data.TLS.ValueBool(),
		"Username": data.Username.ValueString(),
		"Password": data.Password.ValueString(),
	}
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/registries/ping", payload, &result); err != nil {
		resp.Diagnostics.AddError("Failed to test the registry connection", fmt.Errorf("failed to test the registry connection: %w", err).Error())
		return
	}
	if !result.Success && data.FailOnError.ValueBool() {
		resp.Diagnostics.AddError("Registry connection test failed", fmt.Sprintf("the registry connection test failed: %s", result.Message))
		return
	}
	data.Success = types.BoolValue(result.Success)
	data.Message = types.StringValue(result.Message)
	data.ID = types.StringValue("registry-ping-" + data.URL.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
