package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*imageStatusDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*imageStatusDataSource)(nil)
)

// imageStatusDataSource reports whether the image behind a container, a
// service or a whole stack has been superseded in its registry. Portainer has
// three endpoints for it, one per subject, which is what `kind` selects.
func newImageStatusDataSource() datasource.DataSource { return &imageStatusDataSource{} }

type imageStatusDataSource struct{ client *APIClient }

type imageStatusDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	Kind       types.String `tfsdk:"kind"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
	ResourceID types.String `tfsdk:"resource_id"`
	Refresh    types.Bool   `tfsdk:"refresh"`
	Status     types.String `tfsdk:"status"`
	Message    types.String `tfsdk:"message"`
}

func (d *imageStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_image_status"
}

func (d *imageStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, built from `kind`, `endpoint_id` and `resource_id`.",
			},
			"kind": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("container", "service", "stack")},
				Description: "What to check: `container`, `service` or `stack`.",
			},
			"endpoint_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the environment. Required for `container` and `service`; a stack is addressed by its own identifier instead.",
			},
			"resource_id": schema.StringAttribute{
				Required:    true,
				Description: "Identifier of the container, service or stack to check.",
			},
			// Optional+computed so that, as with SDKv2's default, the state
			// holds false when the argument is left out.
			"refresh": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to re-check the registry rather than answer from Portainer's cache. A refresh costs a registry round trip per image.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "What Portainer found: `updated`, `outdated`, `skipped`, `processing`, `preparing` or `error`. Only `outdated` means a newer image is published.",
			},
			"message": schema.StringAttribute{
				Computed:    true,
				Description: "Explanation of the status, empty when there is nothing to say.",
			},
		},
	}
}

func (d *imageStatusDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *imageStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m imageStatusDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Refresh.IsNull() {
		m.Refresh = types.BoolValue(false)
	}
	kind := m.Kind.ValueString()
	resourceID := m.ResourceID.ValueString()
	endpointID := m.EndpointID.ValueInt64()

	var statusURL string
	switch kind {
	case "container", "service":
		if endpointID == 0 {
			resp.Diagnostics.AddError("Missing endpoint_id", fmt.Sprintf("endpoint_id is required when kind is %q", kind))
			return
		}
		statusURL = fmt.Sprintf("%s/docker/%d/%ss/%s/image_status",
			d.client.Endpoint, endpointID, kind, url.PathEscape(resourceID))
	default:
		// A stack is addressed by its own identifier, with no environment in
		// the path.
		statusURL = fmt.Sprintf("%s/stacks/%s/images_status", d.client.Endpoint, url.PathEscape(resourceID))
	}
	if m.Refresh.ValueBool() {
		statusURL += "?refresh=true"
	}

	var response struct {
		Status  string `json:"Status"`
		Message string `json:"Message"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, statusURL, nil, &response); err != nil {
		resp.Diagnostics.AddError("Failed to read the image status", fmt.Errorf("failed to read the image status of %s %s: %w", kind, resourceID, err).Error())
		return
	}

	m.Status = types.StringValue(response.Status)
	m.Message = types.StringValue(response.Message)
	m.ID = types.StringValue(fmt.Sprintf("portainer-image-status-%s-%d-%s", kind, endpointID, resourceID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
