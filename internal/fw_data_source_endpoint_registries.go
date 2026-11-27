package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*endpointRegistriesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointRegistriesDataSource)(nil)
)

func newEndpointRegistriesDataSource() datasource.DataSource { return &endpointRegistriesDataSource{} }

type endpointRegistriesDataSource struct{ client *APIClient }

type endpointRegistriesModel struct {
	ID                     types.String                   `tfsdk:"id"`
	EnvironmentID          types.Int64                    `tfsdk:"environment_id"`
	Namespace              types.String                   `tfsdk:"namespace"`
	DockerhubRegistryID    types.Int64                    `tfsdk:"dockerhub_registry_id"`
	Registries             []endpointRegistriesEntryModel `tfsdk:"registries"`
	DockerhubRateLimit     types.Int64                    `tfsdk:"dockerhub_rate_limit"`
	DockerhubRateRemaining types.Int64                    `tfsdk:"dockerhub_rate_remaining"`
}

type endpointRegistriesEntryModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	URL            types.String `tfsdk:"url"`
	BaseURL        types.String `tfsdk:"base_url"`
	Type           types.Int64  `tfsdk:"type"`
	Authentication types.Bool   `tfsdk:"authentication"`
	Username       types.String `tfsdk:"username"`
}

func (d *endpointRegistriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_registries"
}

func (d *endpointRegistriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Environment (endpoint) identifier whose accessible registries are listed.",
			},
			"namespace": schema.StringAttribute{
				Optional:    true,
				Description: "Kubernetes namespace to scope the list to, for an environment where registry access is granted per namespace.",
			},
			"dockerhub_registry_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of a Docker Hub registry whose pull rate limit should also be fetched. Leave unset to skip that lookup.",
			},
			// Protocol 5 has no nested attributes: a computed list of objects,
			// as SDKv2 declared it.
			"registries": schema.ListAttribute{
				Computed:    true,
				Description: "Registries this environment can pull from.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":             types.Int64Type,
					"name":           types.StringType,
					"url":            types.StringType,
					"base_url":       types.StringType,
					"type":           types.Int64Type,
					"authentication": types.BoolType,
					"username":       types.StringType,
				}},
			},
			"dockerhub_rate_limit": schema.Int64Attribute{
				Computed:    true,
				Description: "Docker Hub pull rate limit for the registry named by `dockerhub_registry_id`, zero when that argument is unset.",
			},
			"dockerhub_rate_remaining": schema.Int64Attribute{
				Computed:    true,
				Description: "Pulls still available within the current window. Watch this before a large rollout that pulls from Docker Hub.",
			},
		},
	}
}

func (d *endpointRegistriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *endpointRegistriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m endpointRegistriesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := m.EnvironmentID.ValueInt64()

	target := fmt.Sprintf("%s/endpoints/%d/registries", d.client.Endpoint, envID)
	if ns := m.Namespace.ValueString(); ns != "" {
		q := url.Values{}
		q.Set("namespace", ns)
		target += "?" + q.Encode()
	}

	// The API returns the full registry object, which carries Password and
	// AccessToken. Those are deliberately not mapped: a data source that copies
	// registry credentials into every state file would be a liability, and
	// nothing in Terraform needs them.
	var list []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		URL            string `json:"URL"`
		BaseURL        string `json:"BaseURL"`
		Type           int    `json:"Type"`
		Authentication bool   `json:"Authentication"`
		Username       string `json:"Username"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, target, nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to list registries", fmt.Errorf("failed to list the registries of environment %d: %w", envID, err).Error())
		return
	}

	m.Registries = make([]endpointRegistriesEntryModel, len(list))
	for i, r := range list {
		m.Registries[i] = endpointRegistriesEntryModel{
			ID:             types.Int64Value(int64(r.ID)),
			Name:           types.StringValue(r.Name),
			URL:            types.StringValue(r.URL),
			BaseURL:        types.StringValue(r.BaseURL),
			Type:           types.Int64Value(int64(r.Type)),
			Authentication: types.BoolValue(r.Authentication),
			Username:       types.StringValue(r.Username),
		}
	}

	limit, remaining := 0, 0
	if hubID := m.DockerhubRegistryID.ValueInt64(); hubID != 0 {
		var status struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		}
		hubURL := fmt.Sprintf("%s/endpoints/%d/dockerhub/%d", d.client.Endpoint, envID, hubID)
		if err := doJSON(ctx, d.client, http.MethodGet, hubURL, nil, &status); err != nil {
			resp.Diagnostics.AddError("Failed to read the Docker Hub rate limit", fmt.Errorf("failed to read the Docker Hub rate limit for registry %d: %w", hubID, err).Error())
			return
		}
		limit, remaining = status.Limit, status.Remaining
	}

	m.DockerhubRateLimit = types.Int64Value(int64(limit))
	m.DockerhubRateRemaining = types.Int64Value(int64(remaining))
	m.ID = types.StringValue(strconv.FormatInt(envID, 10) + "/registries")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
