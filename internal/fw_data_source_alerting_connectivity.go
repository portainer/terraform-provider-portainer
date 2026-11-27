package internal

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*alertingConnectivityDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*alertingConnectivityDataSource)(nil)
)

func newAlertingConnectivityDataSource() datasource.DataSource {
	return &alertingConnectivityDataSource{}
}

type alertingConnectivityDataSource struct{ client *APIClient }

type alertingConnectivityModel struct {
	ID          types.String `tfsdk:"id"`
	URL         types.String `tfsdk:"url"`
	FailOnError types.Bool   `tfsdk:"fail_on_error"`
	Reachable   types.Bool   `tfsdk:"reachable"`
	Error       types.String `tfsdk:"error"`
	Details     types.String `tfsdk:"details"`
}

func (d *alertingConnectivityDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerting_connectivity"
}

func (d *alertingConnectivityDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"url": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Alertmanager to test.",
			},
			// Data sources have no defaults: the attribute is also computed
			// so that an unset value reads back as true, as SDKv2 stored it.
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether an unreachable Alertmanager fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `reachable` instead.",
			},
			"reachable": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer could reach the Alertmanager.",
			},
			"error": schema.StringAttribute{
				Computed:    true,
				Description: "What went wrong, empty when the Alertmanager answered.",
			},
			// Portainer does not describe this response's shape in its own
			// specification, so it is carried through as JSON.
			"details": schema.StringAttribute{
				Computed:    true,
				Description: "The whole response as Portainer returns it, encoded as JSON. Portainer does not describe its shape in the API specification, so decode it with `jsondecode()` if you need more than `reachable`.",
			},
		},
	}
}

func (d *alertingConnectivityDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *alertingConnectivityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data alertingConnectivityModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.FailOnError.IsNull() || data.FailOnError.IsUnknown() {
		data.FailOnError = types.BoolValue(true)
	}
	target := data.URL.ValueString()

	testURL := fmt.Sprintf("%s/observability/alerting/connectivity?url=%s", d.client.Endpoint, url.QueryEscape(target))

	body, err := apiGETRaw(ctx, d.client, testURL)
	reachable := err == nil
	message := ""
	if err != nil {
		// Only what Portainer said is stored, not the error: the message is
		// written to state, and an error carries the whole request URL.
		var statusErr *apiStatusError
		if errors.As(err, &statusErr) {
			message = statusErr.Body
		} else {
			message = "the connectivity check could not be sent"
		}

		if data.FailOnError.ValueBool() {
			resp.Diagnostics.AddError("Alertmanager unreachable", fmt.Sprintf("the Alertmanager at %s could not be reached (set fail_on_error = false to read the result instead): %s", target, message))
			return
		}
		body = nil
	}

	data.Reachable = types.BoolValue(reachable)
	data.Error = types.StringValue(message)
	data.Details = types.StringValue(string(body))
	data.ID = types.StringValue("portainer-alerting-connectivity-" + target)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
