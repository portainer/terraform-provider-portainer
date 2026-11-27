package internal

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*currentUserAuthorizationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*currentUserAuthorizationsDataSource)(nil)
)

func newCurrentUserAuthorizationsDataSource() datasource.DataSource {
	return &currentUserAuthorizationsDataSource{}
}

type currentUserAuthorizationsDataSource struct{ client *APIClient }

type currentUserAuthorizationsDataSourceModel struct {
	ID                      types.String                         `tfsdk:"id"`
	EndpointID              types.Int64                          `tfsdk:"endpoint_id"`
	Authorizations          []string                             `tfsdk:"authorizations"`
	NamespaceAuthorizations []currentUserNamespaceAuthorizations `tfsdk:"namespace_authorizations"`
}

type currentUserNamespaceAuthorizations struct {
	Namespace      string   `tfsdk:"namespace"`
	Authorizations []string `tfsdk:"authorizations"`
}

func (d *currentUserAuthorizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_current_user_authorizations"
}

func (d *currentUserAuthorizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the data source, `portainer-current-user-authorizations-<endpoint_id>`."},
			"endpoint_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the environment to report the current user's authorizations on.",
			},
			"authorizations": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Authorizations the token's user holds on the environment, sorted.",
			},
			"namespace_authorizations": schema.ListAttribute{
				Computed:    true,
				Description: "Per-namespace authorizations. Empty for administrators, for non-Kubernetes environments, and when the user has cluster-wide access.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"namespace":      types.StringType,
					"authorizations": types.ListType{ElemType: types.StringType},
				}},
			},
		},
	}
}

func (d *currentUserAuthorizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *currentUserAuthorizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data currentUserAuthorizationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := data.EndpointID.ValueInt64()

	var environment struct {
		Authorizations map[string]bool `json:"Authorizations"`
	}
	authURL := fmt.Sprintf("%s/users/me/auth/%d", d.client.Endpoint, endpointID)
	if err := doJSON(ctx, d.client, http.MethodGet, authURL, nil, &environment); err != nil {
		resp.Diagnostics.AddError("Failed to read current user authorizations", fmt.Errorf("failed to read the current user's authorizations on environment %d: %w", endpointID, err).Error())
		return
	}

	var perNamespace struct {
		NamespaceAuthorizations map[string]map[string]bool `json:"namespaceAuthorizations"`
	}
	namespacesURL := fmt.Sprintf("%s/users/me/auth/%d/namespaces", d.client.Endpoint, endpointID)
	if err := doJSON(ctx, d.client, http.MethodGet, namespacesURL, nil, &perNamespace); err != nil {
		resp.Diagnostics.AddError("Failed to read current user authorizations", fmt.Errorf("failed to read the current user's namespace authorizations on environment %d: %w", endpointID, err).Error())
		return
	}

	names := make([]string, 0, len(perNamespace.NamespaceAuthorizations))
	for name := range perNamespace.NamespaceAuthorizations {
		names = append(names, name)
	}
	sort.Strings(names)

	data.NamespaceAuthorizations = make([]currentUserNamespaceAuthorizations, 0, len(names))
	for _, name := range names {
		data.NamespaceAuthorizations = append(data.NamespaceAuthorizations, currentUserNamespaceAuthorizations{
			Namespace:      name,
			Authorizations: grantedAuthorizations(perNamespace.NamespaceAuthorizations[name]),
		})
	}
	data.Authorizations = grantedAuthorizations(environment.Authorizations)
	data.ID = types.StringValue(fmt.Sprintf("portainer-current-user-authorizations-%d", endpointID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
