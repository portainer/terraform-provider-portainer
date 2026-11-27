package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*webhookDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*webhookDataSource)(nil)
)

func newWebhookDataSource() datasource.DataSource { return &webhookDataSource{} }

type webhookDataSource struct{ client *APIClient }

type webhookDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	ResourceID  types.String `tfsdk:"resource_id"`
	EndpointID  types.Int64  `tfsdk:"endpoint_id"`
	WebhookType types.Int64  `tfsdk:"webhook_type"`
	Token       types.String `tfsdk:"token"`
}

func (d *webhookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (d *webhookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the webhook.",
			},
			"resource_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the resource (e.g. service) the Portainer webhook is attached to. Combined with endpoint_id to find the webhook.",
			},
			"endpoint_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer environment (endpoint) where the webhook is registered.",
			},
			"webhook_type": schema.Int64Attribute{
				Computed:    true,
				Description: "Type of the Portainer webhook (e.g. service webhook, container webhook).",
			},
			"token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Token used to invoke the Portainer webhook URL.",
			},
		},
	}
}

func (d *webhookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *webhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data webhookDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resourceID := data.ResourceID.ValueString()
	endpointID := int(data.EndpointID.ValueInt64())

	webhooks, err := listWebhooks(ctx, d.client, endpointID, resourceID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read webhook", err.Error())
		return
	}
	for _, w := range webhooks {
		if w.ResourceID == resourceID && w.EndpointID == endpointID {
			data.ID = types.StringValue(strconv.Itoa(w.ID))
			data.WebhookType = types.Int64Value(int64(w.Type))
			data.Token = types.StringValue(w.Token)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("Failed to read webhook", fmt.Sprintf("webhook for resource %s in endpoint %d not found", resourceID, endpointID))
}
