package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*systemDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*systemDataSource)(nil)
)

func newSystemDataSource() datasource.DataSource { return &systemDataSource{} }

type systemDataSource struct{ client *APIClient }

type systemDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	InstanceID         types.String `tfsdk:"instance_id"`
	Version            types.String `tfsdk:"version"`
	ServerVersion      types.String `tfsdk:"server_version"`
	ServerEdition      types.String `tfsdk:"server_edition"`
	DatabaseVersion    types.String `tfsdk:"database_version"`
	LatestVersion      types.String `tfsdk:"latest_version"`
	UpdateAvailable    types.Bool   `tfsdk:"update_available"`
	VersionSupport     types.String `tfsdk:"version_support"`
	Agents             types.Int64  `tfsdk:"agents"`
	EdgeAgents         types.Int64  `tfsdk:"edge_agents"`
	Platform           types.String `tfsdk:"platform"`
	Nodes              types.Int64  `tfsdk:"nodes"`
	AddonsAvailable    types.Bool   `tfsdk:"addons_available"`
	AddonsAvailability types.String `tfsdk:"addons_availability"`
	AdminInitialized   types.Bool   `tfsdk:"admin_initialized"`
}

func (d *systemDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system"
}

func (d *systemDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}
	num := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Computed: true, Description: desc}
	}
	flag := func(desc string) schema.BoolAttribute { return schema.BoolAttribute{Computed: true, Description: desc} }
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"instance_id":         str("Unique identifier of this Portainer instance."),
			"version":             str("Version of the running Portainer instance, as reported by its status endpoint."),
			"server_version":      str("Server version reported by the version endpoint. Use it to gate configuration on the Portainer release, for example a resource that needs 2.45 or newer."),
			"server_edition":      str("Portainer edition as the server reports it: `CE`, or `EE` for Business Edition."),
			"database_version":    str("Schema version of Portainer's database."),
			"latest_version":      str("Newest Portainer version available upstream, empty when the instance cannot reach the update service."),
			"update_available":    flag("Whether a newer Portainer version than the running one is available."),
			"version_support":     str("Support status of the running version, such as `STS` or `LTS`."),
			"agents":              num("Number of standard agents connected to this instance."),
			"edge_agents":         num("Number of Edge agents connected to this instance."),
			"platform":            str("Container platform Portainer itself runs on, such as `docker` or `kubernetes`."),
			"nodes":               num("Total number of nodes across every environment, which is what Portainer licences against."),
			"addons_available":    flag("Whether addons can be installed on this instance."),
			"addons_availability": str("Why addons can or cannot be installed: `available`, `kubernetes-required`, `fips-mode` or `encryption-key-required`. Servers before 3.0 report no reason, only `available` or `unavailable`; empty when the server reports nothing."),
			"admin_initialized":   flag("Whether an administrator account already exists. False means the instance is still awaiting its initial setup."),
		},
	}
}

func (d *systemDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *systemDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	client := d.client

	// /status is the deprecated alias of /system/status and returns the same
	// object, so only the current path is called.
	var status struct {
		InstanceID string `json:"InstanceID"`
		Version    string `json:"Version"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/status", nil, &status); err != nil {
		resp.Diagnostics.AddError("Failed to read the Portainer status", fmt.Sprintf("failed to read the Portainer status: %s", err))
		return
	}

	var version struct {
		ServerVersion   string `json:"ServerVersion"`
		ServerEdition   string `json:"ServerEdition"`
		DatabaseVersion string `json:"DatabaseVersion"`
		LatestVersion   string `json:"LatestVersion"`
		UpdateAvailable bool   `json:"UpdateAvailable"`
		VersionSupport  string `json:"VersionSupport"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version); err != nil {
		resp.Diagnostics.AddError("Failed to read the Portainer version", fmt.Sprintf("failed to read the Portainer version: %s", err))
		return
	}

	var info struct {
		Agents          int                `json:"agents"`
		EdgeAgents      int                `json:"edgeAgents"`
		Platform        string             `json:"platform"`
		AddonsAvailable addonsAvailability `json:"addonsAvailable"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/info", nil, &info); err != nil {
		resp.Diagnostics.AddError("Failed to read the Portainer system info", fmt.Sprintf("failed to read the Portainer system info: %s", err))
		return
	}

	var nodes struct {
		Nodes int `json:"nodes"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/nodes", nil, &nodes); err != nil {
		resp.Diagnostics.AddError("Failed to read the Portainer node count", fmt.Sprintf("failed to read the Portainer node count: %s", err))
		return
	}

	// The admin check answers 204 when an administrator exists and 404 when the
	// instance is still uninitialised, so the status code is the answer.
	adminErr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/users/admin/check", nil, nil)
	adminInitialized := adminErr == nil
	if adminErr != nil && !isAPINotFound(adminErr) {
		resp.Diagnostics.AddError("Failed to check for an administrator account", fmt.Sprintf("failed to check for an administrator account: %s", adminErr))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &systemDataSourceModel{
		ID:                 types.StringValue("portainer-system-" + status.InstanceID),
		InstanceID:         types.StringValue(status.InstanceID),
		Version:            types.StringValue(status.Version),
		ServerVersion:      types.StringValue(version.ServerVersion),
		ServerEdition:      types.StringValue(version.ServerEdition),
		DatabaseVersion:    types.StringValue(version.DatabaseVersion),
		LatestVersion:      types.StringValue(version.LatestVersion),
		UpdateAvailable:    types.BoolValue(version.UpdateAvailable),
		VersionSupport:     types.StringValue(version.VersionSupport),
		Agents:             types.Int64Value(int64(info.Agents)),
		EdgeAgents:         types.Int64Value(int64(info.EdgeAgents)),
		Platform:           types.StringValue(info.Platform),
		Nodes:              types.Int64Value(int64(nodes.Nodes)),
		AddonsAvailable:    types.BoolValue(info.AddonsAvailable == addonsAvailable),
		AddonsAvailability: types.StringValue(string(info.AddonsAvailable)),
		AdminInitialized:   types.BoolValue(adminInitialized),
	})...)
}

// addonsAvailable is the addonsAvailability of an instance addons can be
// installed on.
const addonsAvailable = "available"

// addonsAvailability is system/info's addonsAvailable. Portainer 3.0 turned it
// from a bool into a string naming the failing gate (`kubernetes-required`,
// `fips-mode`, `encryption-key-required`); a 2.x bool is mapped to
// `available` or `unavailable`, so both decode into the same value.
type addonsAvailability string

func (a *addonsAvailability) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*a = ""
		return nil
	}
	var available bool
	if err := json.Unmarshal(raw, &available); err == nil {
		*a = "unavailable"
		if available {
			*a = addonsAvailable
		}
		return nil
	}
	var reason string
	if err := json.Unmarshal(raw, &reason); err != nil {
		return fmt.Errorf("addonsAvailable: expected a bool or a string, got %s", raw)
	}
	*a = addonsAvailability(reason)
	return nil
}
