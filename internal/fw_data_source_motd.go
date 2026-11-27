package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*motdDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*motdDataSource)(nil)
)

func newMOTDDataSource() datasource.DataSource { return &motdDataSource{} }

type motdDataSource struct{ client *APIClient }

type motdDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Title   types.String `tfsdk:"title"`
	Message types.String `tfsdk:"message"`
	Style   types.String `tfsdk:"style"`
	Hash    types.String `tfsdk:"hash"`
}

func (d *motdDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_motd"
}

func (d *motdDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":      schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"title":   schema.StringAttribute{Computed: true, Description: "Title of the message of the day, empty when Portainer is serving none."},
			"message": schema.StringAttribute{Computed: true, Description: "Body of the message of the day."},
			"style":   schema.StringAttribute{Computed: true, Description: "Inline CSS Portainer's UI applies to the message."},
			"hash":    schema.StringAttribute{Computed: true, Description: "Hash of the message, which the UI uses to remember that a user has dismissed this particular one. Changes whenever the message changes."},
		},
	}
}

func (d *motdDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *motdDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Hash is []byte on the Portainer side, so it arrives base64-encoded and is
	// kept as the opaque string it is.
	var motd struct {
		Title   string `json:"Title"`
		Message string `json:"Message"`
		Style   string `json:"Style"`
		Hash    string `json:"Hash"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/motd", nil, &motd); err != nil {
		resp.Diagnostics.AddError("Failed to read the message of the day", fmt.Sprintf("failed to read the message of the day: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &motdDataSourceModel{
		ID:      types.StringValue("portainer-motd"),
		Title:   types.StringValue(motd.Title),
		Message: types.StringValue(motd.Message),
		Style:   types.StringValue(motd.Style),
		Hash:    types.StringValue(motd.Hash),
	})...)
}
