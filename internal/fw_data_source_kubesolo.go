package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The KubeSolo install flow itself (POST /kubesolo/tests, /kubesolo/install
// and the retry/cancel actions) is not modelled: it is an asynchronous SSH job
// over many machines that Portainer holds one of at a time, with host key
// confirmation in between, which no Terraform resource lifecycle fits. The
// data sources below read what that flow and a running KubeSolo report.

// kubeSoloOfflineBundle is one uploaded offline tarball, as listed by GET
// /kubesolo/offline-bundles and returned by the upload.
type kubeSoloOfflineBundle struct {
	Name string `json:"name"`
	Arch string `json:"arch"`
	Libc string `json:"libc"`
	Size int64  `json:"size"`
}

// listKubeSoloOfflineBundles returns every uploaded KubeSolo offline bundle.
func listKubeSoloOfflineBundles(ctx context.Context, client *APIClient) ([]kubeSoloOfflineBundle, error) {
	var bundles []kubeSoloOfflineBundle
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/kubesolo/offline-bundles", nil, &bundles); err != nil {
		return nil, err
	}
	return bundles, nil
}

// --- portainer_kubesolo_versions ---

var _ datasource.DataSourceWithConfigure = (*kubeSoloVersionsDataSource)(nil)

func newKubeSoloVersionsDataSource() datasource.DataSource { return &kubeSoloVersionsDataSource{} }

type kubeSoloVersionsDataSource struct{ edgeDataSourceBase }

type kubeSoloVersionsModel struct {
	ID          types.String `tfsdk:"id"`
	Bundled     types.String `tfsdk:"bundled"`
	Minimum     types.String `tfsdk:"minimum"`
	Versions    types.List   `tfsdk:"versions"`
	GithubError types.String `tfsdk:"github_error"`
}

func (d *kubeSoloVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubesolo_versions"
}

func (d *kubeSoloVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-kubesolo-versions`."},
			"bundled": schema.StringAttribute{
				Computed:    true,
				Description: "KubeSolo release Portainer was built with. It is installed by default and on machines that cannot reach the internet.",
			},
			"minimum": schema.StringAttribute{
				Computed:    true,
				Description: "Oldest KubeSolo release Portainer installs.",
			},
			"versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "KubeSolo releases Portainer can install, newest first. Only the bundled one when GitHub cannot be reached.",
			},
			"github_error": schema.StringAttribute{
				Computed:    true,
				Description: "Why Portainer could not list the releases on GitHub, empty when it could.",
			},
		},
	}
}

func (d *kubeSoloVersionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var versions struct {
		Bundled     string   `json:"bundled"`
		Minimum     string   `json:"minimum"`
		Versions    []string `json:"versions"`
		GithubError string   `json:"githubError"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/kubesolo/versions", nil, &versions); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_kubesolo_versions", requiresBE30)
		resp.Diagnostics.AddError("Failed to list KubeSolo versions", fmt.Sprintf("failed to list KubeSolo versions: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &kubeSoloVersionsModel{
		ID:          types.StringValue("portainer-kubesolo-versions"),
		Bundled:     types.StringValue(versions.Bundled),
		Minimum:     types.StringValue(versions.Minimum),
		Versions:    edgeStringList(versions.Versions),
		GithubError: types.StringValue(versions.GithubError),
	})...)
}

// --- portainer_kubesolo_offline_bundles ---

var _ datasource.DataSourceWithConfigure = (*kubeSoloOfflineBundlesDataSource)(nil)

func newKubeSoloOfflineBundlesDataSource() datasource.DataSource {
	return &kubeSoloOfflineBundlesDataSource{}
}

type kubeSoloOfflineBundlesDataSource struct{ edgeDataSourceBase }

var kubeSoloBundleAttrTypes = map[string]attr.Type{
	"name": types.StringType, "arch": types.StringType, "libc": types.StringType, "size": types.Int64Type,
}

func (d *kubeSoloOfflineBundlesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubesolo_offline_bundles"
}

func (d *kubeSoloOfflineBundlesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Always `portainer-kubesolo-offline-bundles`."},
			"bundles": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ObjectType{AttrTypes: kubeSoloBundleAttrTypes},
				Description: "Offline tarballs of the bundled KubeSolo release uploaded for machines that cannot reach the internet.",
			},
		},
	}
}

func (d *kubeSoloOfflineBundlesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	bundles, err := listKubeSoloOfflineBundles(ctx, d.client)
	if err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_kubesolo_offline_bundles", requiresBE30)
		resp.Diagnostics.AddError("Failed to list KubeSolo offline bundles", fmt.Sprintf("failed to list KubeSolo offline bundles: %s", err))
		return
	}
	items := make([]map[string]attr.Value, 0, len(bundles))
	for _, b := range bundles {
		items = append(items, map[string]attr.Value{
			"name": types.StringValue(b.Name), "arch": types.StringValue(b.Arch),
			"libc": types.StringValue(b.Libc), "size": types.Int64Value(b.Size),
		})
	}
	list := edgeObjectList(kubeSoloBundleAttrTypes, items, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bundles"), list)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue("portainer-kubesolo-offline-bundles"))...)
}

// --- portainer_kubesolo_config ---

var _ datasource.DataSourceWithConfigure = (*kubeSoloConfigDataSource)(nil)

func newKubeSoloConfigDataSource() datasource.DataSource { return &kubeSoloConfigDataSource{} }

type kubeSoloConfigDataSource struct{ edgeDataSourceBase }

type kubeSoloConfigModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	API           types.String `tfsdk:"api"`
	Version       types.String `tfsdk:"version"`
	ConfigJSON    types.String `tfsdk:"config_json"`
	Error         types.String `tfsdk:"error"`
	ReadAt        types.Int64  `tfsdk:"read_at"`
}

func (d *kubeSoloConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubesolo_config"
}

func (d *kubeSoloConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the environment, as a string."},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the KubeSolo environment (a Kubernetes Edge Agent environment).",
			},
			"api": schema.StringAttribute{
				Computed:    true,
				Description: "Whether the agent could reach the KubeSolo API: `available`, `notConfigured` or `unreachable`.",
			},
			"version": schema.StringAttribute{
				Computed:    true,
				Description: "KubeSolo version running on the machine.",
			},
			"config_json": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "KubeSolo configuration as a JSON object. Sensitive: it carries KubeSolo's secrets, such as the Edge key. Decode it with `jsondecode`.",
			},
			"error": schema.StringAttribute{
				Computed:    true,
				Description: "Why the configuration could not be read, empty when it could.",
			},
			"read_at": schema.Int64Attribute{
				Computed:    true,
				Description: "When the snapshot that carried the configuration was taken, as a Unix timestamp.",
			},
		},
	}
}

func (d *kubeSoloConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m kubeSoloConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := m.EnvironmentID.ValueInt64()

	// The configuration comes from the environment's latest snapshot, so a
	// 404 can also mean the agent has not sent one with KubeSolo's state yet.
	var body struct {
		API     string          `json:"api"`
		Version string          `json:"version"`
		Config  json.RawMessage `json:"config"`
		Error   string          `json:"error"`
		ReadAt  int64           `json:"readAt"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/kubesolo/environments/%d/config", d.client.Endpoint, envID), nil, &body); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_kubesolo_config", requiresBE30)
		resp.Diagnostics.AddError("Failed to read the KubeSolo configuration", fmt.Sprintf("failed to read the KubeSolo configuration of environment %d: %s", envID, err))
		return
	}

	config := ""
	if len(body.Config) > 0 && string(body.Config) != "null" {
		config = string(body.Config)
	}
	m.ID = types.StringValue(strconv.FormatInt(envID, 10))
	m.API = types.StringValue(body.API)
	m.Version = types.StringValue(body.Version)
	m.ConfigJSON = types.StringValue(config)
	m.Error = types.StringValue(body.Error)
	m.ReadAt = types.Int64Value(body.ReadAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
