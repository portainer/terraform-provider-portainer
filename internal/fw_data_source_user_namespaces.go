package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*userNamespacesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userNamespacesDataSource)(nil)
)

func newUserNamespacesDataSource() datasource.DataSource { return &userNamespacesDataSource{} }

type userNamespacesDataSource struct{ client *APIClient }

type userNamespacesDataSourceModel struct {
	ID         types.String               `tfsdk:"id"`
	UserID     types.Int64                `tfsdk:"user_id"`
	Namespaces []userNamespacesEntryModel `tfsdk:"namespaces"`
	Raw        types.String               `tfsdk:"raw"`
}

type userNamespacesEntryModel struct {
	EndpointID     string   `tfsdk:"endpoint_id"`
	Namespace      string   `tfsdk:"namespace"`
	Authorizations []string `tfsdk:"authorizations"`
}

func (d *userNamespacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_namespaces"
}

func (d *userNamespacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the data source, `portainer-user-namespaces-<user_id>`."},
			"user_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the user whose namespace authorizations are read.",
			},
			"namespaces": schema.ListAttribute{
				Computed:    true,
				Description: "The user's namespace authorizations, one entry per environment and namespace.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"endpoint_id":    types.StringType,
					"namespace":      types.StringType,
					"authorizations": types.ListType{ElemType: types.StringType},
				}},
			},
			"raw": schema.StringAttribute{
				Computed:    true,
				Description: "The whole response as Portainer returns it, encoded as JSON. Portainer adds authorization names between releases, so this is what to read when a name is not in the flattened list yet.",
			},
		},
	}
}

func (d *userNamespacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *userNamespacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userNamespacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := data.UserID.ValueInt64()

	// The response is a map of environment identifier to a map of namespace to
	// its authorizations.
	var mapping map[string]map[string]map[string]bool
	namespacesURL := fmt.Sprintf("%s/users/%d/namespaces", d.client.Endpoint, userID)
	body, err := apiGETRaw(ctx, d.client, namespacesURL)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read user namespaces", fmt.Errorf("failed to read the namespace authorizations of user %d: %w", userID, err).Error())
		return
	}
	if err := json.Unmarshal(body, &mapping); err != nil {
		resp.Diagnostics.AddError("Failed to read user namespaces", fmt.Errorf("failed to parse the namespace authorizations of user %d: %w", userID, err).Error())
		return
	}

	// Both levels are sorted so two identical plans produce identical output;
	// Go iterates a map in a random order.
	endpointIDs := make([]string, 0, len(mapping))
	for endpointID := range mapping {
		endpointIDs = append(endpointIDs, endpointID)
	}
	sort.Strings(endpointIDs)

	data.Namespaces = []userNamespacesEntryModel{}
	for _, endpointID := range endpointIDs {
		namespaces := mapping[endpointID]
		names := make([]string, 0, len(namespaces))
		for name := range namespaces {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			data.Namespaces = append(data.Namespaces, userNamespacesEntryModel{
				EndpointID:     endpointID,
				Namespace:      name,
				Authorizations: grantedAuthorizations(namespaces[name]),
			})
		}
	}

	data.Raw = types.StringValue(string(body))
	data.ID = types.StringValue(fmt.Sprintf("portainer-user-namespaces-%d", userID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
