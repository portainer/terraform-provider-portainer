package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The Omni data sources. Their computed lists of objects were attributes
// (not blocks) under SDKv2 too, so they are list attributes of objects here.

var (
	_ datasource.DataSourceWithConfigure = (*omniMachinesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*omniMachineDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*omniMachineLogsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*omniTalosVersionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*omniUpgradeStatusDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*omniServiceAccountDataSource)(nil)
)

//nolint:gosec // a description, not a credential
const omniCredentialIDDescription = "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key."

func omniCredentialIDAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{Required: true, Description: omniCredentialIDDescription}
}

func omniComputedString(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: description}
}

func omniComputedBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Computed: true, Description: description}
}

func omniComputedStrings(description string) schema.ListAttribute {
	return schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: description}
}

// omniDataSourceBase carries the client every Omni data source needs.
type omniDataSourceBase struct{ client *APIClient }

func (b *omniDataSourceBase) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	b.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// --- portainer_omni_machines -------------------------------------------------

var omniMachineSummaryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"machine_name":       types.StringType,
	"cluster":            types.StringType,
	"connected":          types.BoolType,
	"maintenance":        types.BoolType,
	"management_address": types.StringType,
	"power_state":        types.StringType,
	"role":               types.StringType,
	"talos_version":      types.StringType,
	"last_error":         types.StringType,
	"labels":             types.MapType{ElemType: types.StringType},
}}

// omniMachineSummaryAttrs is the subset both machine data sources agree on.
// Listing is for finding a machine; the inspect data source is where the
// detail lives.
func omniMachineSummaryAttrs(machine omniMachineStatus) map[string]attr.Value {
	labels := machine.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return map[string]attr.Value{
		"machine_name":       types.StringValue(machine.MachineName),
		"cluster":            types.StringValue(machine.Spec.Cluster),
		"connected":          types.BoolValue(machine.Spec.Connected),
		"maintenance":        types.BoolValue(machine.Spec.Maintenance),
		"management_address": types.StringValue(machine.Spec.ManagementAddress),
		"power_state":        types.StringValue(machine.Spec.PowerState),
		"role":               types.StringValue(machine.Spec.Role),
		"talos_version":      types.StringValue(machine.Spec.TalosVersion),
		"last_error":         types.StringValue(machine.Spec.LastError),
		"labels":             stringMapValue(labels),
	}
}

func newOmniMachinesDataSource() datasource.DataSource { return &omniMachinesDataSource{} }

type omniMachinesDataSource struct{ omniDataSourceBase }

type omniMachinesModel struct {
	ID           types.String `tfsdk:"id"`
	CredentialID types.Int64  `tfsdk:"credential_id"`
	Machines     types.List   `tfsdk:"machines"`
}

func (d *omniMachinesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_machines"
}

func (d *omniMachinesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"credential_id": omniCredentialIDAttribute(),
			"machines": schema.ListAttribute{
				Computed:    true,
				ElementType: omniMachineSummaryType,
				Description: "The machines Omni knows about. Use `portainer_omni_machine` for the hardware and network detail of one of them.",
			},
		},
	}
}

func (d *omniMachinesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniMachinesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID := m.CredentialID.ValueInt64()

	var machines []omniMachineStatus
	listURL := fmt.Sprintf("%s/omni/%d/machines", d.client.Endpoint, credentialID)
	if err := doJSON(ctx, d.client, http.MethodGet, listURL, nil, &machines); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to list the Omni machines: %w", err))
		return
	}

	summaries := make([]attr.Value, 0, len(machines))
	for _, machine := range machines {
		summaries = append(summaries, types.ObjectValueMust(omniMachineSummaryType.AttrTypes, omniMachineSummaryAttrs(machine)))
	}
	m.Machines = types.ListValueMust(omniMachineSummaryType, summaries)
	m.ID = types.StringValue(fmt.Sprintf("portainer-omni-machines-%d", credentialID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_omni_machine --------------------------------------------------

var (
	omniBlockDeviceType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"linux_name": types.StringType, "model": types.StringType, "size": types.Int64Type,
		"type": types.StringType, "system_disk": types.BoolType,
	}}
	omniMemoryModuleType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"description": types.StringType, "size_mb": types.Int64Type,
	}}
	omniProcessorType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"description": types.StringType, "manufacturer": types.StringType,
		"core_count": types.Int64Type, "thread_count": types.Int64Type, "frequency": types.Int64Type,
	}}
	omniNetworkLinkType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"linux_name": types.StringType, "hardware_address": types.StringType,
		"link_up": types.BoolType, "speed_mbps": types.Int64Type,
	}}
)

func newOmniMachineDataSource() datasource.DataSource { return &omniMachineDataSource{} }

type omniMachineDataSource struct{ omniDataSourceBase }

type omniMachineModel struct {
	ID                types.String `tfsdk:"id"`
	CredentialID      types.Int64  `tfsdk:"credential_id"`
	MachineName       types.String `tfsdk:"machine_name"`
	Cluster           types.String `tfsdk:"cluster"`
	Connected         types.Bool   `tfsdk:"connected"`
	Maintenance       types.Bool   `tfsdk:"maintenance"`
	ManagementAddress types.String `tfsdk:"management_address"`
	PowerState        types.String `tfsdk:"power_state"`
	Role              types.String `tfsdk:"role"`
	TalosVersion      types.String `tfsdk:"talos_version"`
	LastError         types.String `tfsdk:"last_error"`
	Labels            types.Map    `tfsdk:"labels"`
	Architecture      types.String `tfsdk:"architecture"`
	Hostname          types.String `tfsdk:"hostname"`
	DomainName        types.String `tfsdk:"domain_name"`
	Addresses         types.List   `tfsdk:"addresses"`
	DefaultGateways   types.List   `tfsdk:"default_gateways"`
	Platform          types.String `tfsdk:"platform"`
	InstanceID        types.String `tfsdk:"instance_id"`
	InstanceType      types.String `tfsdk:"instance_type"`
	Region            types.String `tfsdk:"region"`
	Zone              types.String `tfsdk:"zone"`
	BlockDevices      types.List   `tfsdk:"block_devices"`
	MemoryModules     types.List   `tfsdk:"memory_modules"`
	Processors        types.List   `tfsdk:"processors"`
	NetworkLinks      types.List   `tfsdk:"network_links"`
}

func (d *omniMachineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_machine"
}

func (d *omniMachineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"credential_id": omniCredentialIDAttribute(),
			"machine_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the machine to inspect.",
			},
			"cluster":            omniComputedString("Cluster the machine belongs to, empty while it is unallocated."),
			"connected":          omniComputedBool("Whether the machine is currently connected to Omni."),
			"maintenance":        omniComputedBool("Whether the machine is in maintenance mode."),
			"management_address": omniComputedString("Address Omni manages the machine on."),
			"power_state":        omniComputedString("Power state Omni reports for the machine."),
			"role":               omniComputedString("Role the machine plays in its cluster."),
			"talos_version":      omniComputedString("Talos version running on the machine."),
			"last_error":         omniComputedString("Most recent error Omni recorded for the machine, empty when there is none."),
			"labels": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Labels Omni has on the machine.",
			},
			"architecture":     omniComputedString("CPU architecture of the machine."),
			"hostname":         omniComputedString("Hostname the machine reports."),
			"domain_name":      omniComputedString("Domain name the machine reports."),
			"addresses":        omniComputedStrings("Addresses configured on the machine."),
			"default_gateways": omniComputedStrings("Default gateways configured on the machine."),
			"platform":         omniComputedString("Platform the machine runs on, as reported by its metadata."),
			"instance_id":      omniComputedString("Instance identifier from the platform metadata."),
			"instance_type":    omniComputedString("Instance type from the platform metadata."),
			"region":           omniComputedString("Region from the platform metadata."),
			"zone":             omniComputedString("Zone from the platform metadata."),
			"block_devices": schema.ListAttribute{
				Computed:    true,
				ElementType: omniBlockDeviceType,
				Description: "Block devices Omni sees on the machine, which is where an install disk name comes from.",
			},
			"memory_modules": schema.ListAttribute{
				Computed:    true,
				ElementType: omniMemoryModuleType,
				Description: "Memory modules Omni sees on the machine.",
			},
			"processors": schema.ListAttribute{
				Computed:    true,
				ElementType: omniProcessorType,
				Description: "Processors Omni sees on the machine.",
			},
			"network_links": schema.ListAttribute{
				Computed:    true,
				ElementType: omniNetworkLinkType,
				Description: "Network links Omni sees on the machine, which is where an interface name comes from.",
			},
		},
	}
}

func (d *omniMachineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniMachineModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID := m.CredentialID.ValueInt64()
	name := m.MachineName.ValueString()

	var machine omniMachineStatus
	machineURL := fmt.Sprintf("%s/omni/%d/machine/%s", d.client.Endpoint, credentialID, url.PathEscape(name))
	if err := doJSON(ctx, d.client, http.MethodGet, machineURL, nil, &machine); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to read Omni machine %s: %w", name, err))
		return
	}

	summary := omniMachineSummaryAttrs(machine)
	m.Cluster = summary["cluster"].(types.String)
	m.Connected = summary["connected"].(types.Bool)
	m.Maintenance = summary["maintenance"].(types.Bool)
	m.ManagementAddress = summary["management_address"].(types.String)
	m.PowerState = summary["power_state"].(types.String)
	m.Role = summary["role"].(types.String)
	m.TalosVersion = summary["talos_version"].(types.String)
	m.LastError = summary["last_error"].(types.String)
	m.Labels = summary["labels"].(types.Map)
	m.Architecture = types.StringValue(machine.Spec.Hardware.Arch)
	m.Hostname = types.StringValue(machine.Spec.Network.Hostname)
	m.DomainName = types.StringValue(machine.Spec.Network.DomainName)
	m.Addresses = omniStringList(machine.Spec.Network.Addresses)
	m.DefaultGateways = omniStringList(machine.Spec.Network.DefaultGateways)
	m.Platform = types.StringValue(machine.Spec.PlatformMetadata.Platform)
	m.InstanceID = types.StringValue(machine.Spec.PlatformMetadata.InstanceID)
	m.InstanceType = types.StringValue(machine.Spec.PlatformMetadata.InstanceType)
	m.Region = types.StringValue(machine.Spec.PlatformMetadata.Region)
	m.Zone = types.StringValue(machine.Spec.PlatformMetadata.Zone)

	devices := make([]attr.Value, 0, len(machine.Spec.Hardware.BlockDevices))
	for _, device := range machine.Spec.Hardware.BlockDevices {
		devices = append(devices, types.ObjectValueMust(omniBlockDeviceType.AttrTypes, map[string]attr.Value{
			"linux_name": types.StringValue(device.LinuxName), "model": types.StringValue(device.Model),
			"size": types.Int64Value(device.Size), "type": types.StringValue(device.Type),
			"system_disk": types.BoolValue(device.SystemDisk),
		}))
	}
	m.BlockDevices = types.ListValueMust(omniBlockDeviceType, devices)

	modules := make([]attr.Value, 0, len(machine.Spec.Hardware.MemoryModules))
	for _, module := range machine.Spec.Hardware.MemoryModules {
		modules = append(modules, types.ObjectValueMust(omniMemoryModuleType.AttrTypes, map[string]attr.Value{
			"description": types.StringValue(module.Description), "size_mb": types.Int64Value(int64(module.SizeMB)),
		}))
	}
	m.MemoryModules = types.ListValueMust(omniMemoryModuleType, modules)

	processors := make([]attr.Value, 0, len(machine.Spec.Hardware.Processors))
	for _, processor := range machine.Spec.Hardware.Processors {
		processors = append(processors, types.ObjectValueMust(omniProcessorType.AttrTypes, map[string]attr.Value{
			"description": types.StringValue(processor.Description), "manufacturer": types.StringValue(processor.Manufacturer),
			"core_count": types.Int64Value(int64(processor.CoreCount)), "thread_count": types.Int64Value(int64(processor.ThreadCount)),
			"frequency": types.Int64Value(int64(processor.Frequency)),
		}))
	}
	m.Processors = types.ListValueMust(omniProcessorType, processors)

	links := make([]attr.Value, 0, len(machine.Spec.Network.NetworkLinks))
	for _, link := range machine.Spec.Network.NetworkLinks {
		links = append(links, types.ObjectValueMust(omniNetworkLinkType.AttrTypes, map[string]attr.Value{
			"linux_name": types.StringValue(link.LinuxName), "hardware_address": types.StringValue(link.HardwareAddress),
			"link_up": types.BoolValue(link.LinkUp), "speed_mbps": types.Int64Value(int64(link.SpeedMbps)),
		}))
	}
	m.NetworkLinks = types.ListValueMust(omniNetworkLinkType, links)

	m.ID = types.StringValue(fmt.Sprintf("portainer-omni-machine-%d-%s", credentialID, name))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_omni_machine_logs ---------------------------------------------

func newOmniMachineLogsDataSource() datasource.DataSource { return &omniMachineLogsDataSource{} }

type omniMachineLogsDataSource struct{ omniDataSourceBase }

type omniMachineLogsModel struct {
	ID           types.String `tfsdk:"id"`
	CredentialID types.Int64  `tfsdk:"credential_id"`
	MachineName  types.String `tfsdk:"machine_name"`
	Logs         types.String `tfsdk:"logs"`
}

func (d *omniMachineLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_machine_logs"
}

func (d *omniMachineLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"credential_id": omniCredentialIDAttribute(),
			"machine_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the machine to fetch logs for.",
			},
			"logs": omniComputedString("The machine's logs as Omni returns them. They change on every read, so a configuration that depends on this value will show a difference on every plan."),
		},
	}
}

func (d *omniMachineLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniMachineLogsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID := m.CredentialID.ValueInt64()
	name := m.MachineName.ValueString()

	// The endpoint answers with the log text itself rather than a JSON
	// document, so the raw body is what gets read.
	logsURL := fmt.Sprintf("%s/omni/%d/machine/%s/logs", d.client.Endpoint, credentialID, url.PathEscape(name))
	body, err := apiGETRaw(ctx, d.client, logsURL)
	if err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to read the logs of Omni machine %s: %w", name, err))
		return
	}

	m.Logs = types.StringValue(string(body))
	m.ID = types.StringValue(fmt.Sprintf("portainer-omni-machine-logs-%d-%s", credentialID, name))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_omni_talos_versions -------------------------------------------

var omniCompatibilityType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"talos_version":       types.StringType,
	"kubernetes_versions": types.ListType{ElemType: types.StringType},
}}

func newOmniTalosVersionsDataSource() datasource.DataSource { return &omniTalosVersionsDataSource{} }

type omniTalosVersionsDataSource struct{ omniDataSourceBase }

type omniTalosVersionsModel struct {
	ID            types.String `tfsdk:"id"`
	CredentialID  types.Int64  `tfsdk:"credential_id"`
	TalosVersions types.List   `tfsdk:"talos_versions"`
	Compatibility types.List   `tfsdk:"compatibility"`
}

func (d *omniTalosVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_talos_versions"
}

func (d *omniTalosVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"credential_id":  omniCredentialIDAttribute(),
			"talos_versions": omniComputedStrings("The Talos versions the credential can provision, sorted."),
			"compatibility": schema.ListAttribute{
				Computed:    true,
				ElementType: omniCompatibilityType,
				Description: "For each Talos version, the versions Portainer reports as compatible with it.",
			},
		},
	}
}

func (d *omniTalosVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniTalosVersionsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID := m.CredentialID.ValueInt64()

	// Portainer answers with a map of version to the versions it lists against
	// it, so the keys are sorted to keep the data source's output stable.
	var versions map[string][]string
	versionsURL := fmt.Sprintf("%s/omni/%d/cluster/version/talos", d.client.Endpoint, credentialID)
	if err := doJSON(ctx, d.client, http.MethodGet, versionsURL, nil, &versions); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to read the supported Talos versions: %w", err))
		return
	}

	talosVersions := make([]string, 0, len(versions))
	for version := range versions {
		talosVersions = append(talosVersions, version)
	}
	sort.Strings(talosVersions)

	compatibility := make([]attr.Value, 0, len(talosVersions))
	for _, version := range talosVersions {
		compatibility = append(compatibility, types.ObjectValueMust(omniCompatibilityType.AttrTypes, map[string]attr.Value{
			"talos_version":       types.StringValue(version),
			"kubernetes_versions": omniStringList(versions[version]),
		}))
	}

	m.TalosVersions = omniStringList(talosVersions)
	m.Compatibility = types.ListValueMust(omniCompatibilityType, compatibility)
	m.ID = types.StringValue(fmt.Sprintf("portainer-omni-talos-versions-%d", credentialID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_omni_upgrade_status -------------------------------------------

func newOmniUpgradeStatusDataSource() datasource.DataSource { return &omniUpgradeStatusDataSource{} }

type omniUpgradeStatusDataSource struct{ omniDataSourceBase }

type omniUpgradeStatusModel struct {
	ID                types.String `tfsdk:"id"`
	CredentialID      types.Int64  `tfsdk:"credential_id"`
	Cluster           types.String `tfsdk:"cluster"`
	Component         types.String `tfsdk:"component"`
	Status            types.String `tfsdk:"status"`
	Step              types.String `tfsdk:"step"`
	Phase             types.Int64  `tfsdk:"phase"`
	Error             types.String `tfsdk:"error"`
	CurrentVersion    types.String `tfsdk:"current_version"`
	LastVersion       types.String `tfsdk:"last_version"`
	AvailableVersions types.List   `tfsdk:"available_versions"`
}

func (d *omniUpgradeStatusDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_upgrade_status"
}

func (d *omniUpgradeStatusDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"credential_id": omniCredentialIDAttribute(),
			"cluster": schema.StringAttribute{
				Required:    true,
				Description: "Name of the cluster to report on.",
			},
			"component": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.OneOf("kubernetes", "talos")},
				Description: "Which upgrade to report on: `kubernetes` or `talos`. Portainer tracks the two separately and they can be at different steps.",
			},
			"status": omniComputedString("Status Omni reports for the upgrade."),
			"step":   omniComputedString("Step the upgrade is currently on."),
			"phase": schema.Int64Attribute{
				Computed:    true,
				Description: "Phase Omni reports for the upgrade.",
			},
			"error":              omniComputedString("Why the upgrade failed, empty when it has not."),
			"current_version":    omniComputedString("Version the upgrade is moving to."),
			"last_version":       omniComputedString("Version the cluster was on before the upgrade."),
			"available_versions": omniComputedStrings("Versions the cluster can be upgraded to."),
		},
	}
}

func (d *omniUpgradeStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniUpgradeStatusModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	credentialID := m.CredentialID.ValueInt64()
	cluster := m.Cluster.ValueString()
	component := m.Component.ValueString()

	// Portainer spells the Kubernetes half of this pair "k8s" in the path.
	suffix := "talos"
	if component == "kubernetes" {
		suffix = "k8s"
	}

	var status struct {
		Status                string   `json:"status"`
		Step                  string   `json:"step"`
		Phase                 int      `json:"phase"`
		Error                 string   `json:"error"`
		CurrentUpgradeVersion string   `json:"current_upgrade_version"`
		LastUpgradeVersion    string   `json:"last_upgrade_version"`
		UpgradeVersions       []string `json:"upgrade_versions"`
	}
	statusURL := fmt.Sprintf("%s/omni/%d/cluster/%s/upgrade/status/%s",
		d.client.Endpoint, credentialID, url.PathEscape(cluster), suffix)
	if err := doJSON(ctx, d.client, http.MethodGet, statusURL, nil, &status); err != nil {
		omniAddError(&resp.Diagnostics, fmt.Errorf("failed to read the %s upgrade status of Omni cluster %s: %w", component, cluster, err))
		return
	}

	m.Status = types.StringValue(status.Status)
	m.Step = types.StringValue(status.Step)
	m.Phase = types.Int64Value(int64(status.Phase))
	m.Error = types.StringValue(status.Error)
	m.CurrentVersion = types.StringValue(status.CurrentUpgradeVersion)
	m.LastVersion = types.StringValue(status.LastUpgradeVersion)
	m.AvailableVersions = omniStringList(status.UpgradeVersions)
	m.ID = types.StringValue(fmt.Sprintf("portainer-omni-upgrade-status-%d-%s-%s", credentialID, cluster, component))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- portainer_omni_service_account ------------------------------------------

func newOmniServiceAccountDataSource() datasource.DataSource { return &omniServiceAccountDataSource{} }

type omniServiceAccountDataSource struct{ omniDataSourceBase }

type omniServiceAccountModel struct {
	ID                types.String `tfsdk:"id"`
	Endpoint          types.String `tfsdk:"endpoint"`
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
	FailOnError       types.Bool   `tfsdk:"fail_on_error"`
	Valid             types.Bool   `tfsdk:"valid"`
	Error             types.String `tfsdk:"error"`
}

func (d *omniServiceAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_service_account"
}

func (d *omniServiceAccountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"endpoint": schema.StringAttribute{
				Required:    true,
				Description: "URL of the Omni instance to check, the value of `OMNI_ENDPOINT`.",
			},
			"service_account_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Omni service account key to check, the value of `OMNI_SERVICE_ACCOUNT_KEY`.",
			},
			// Defaults to true (data sources have no Default: unset reads as
			// true) so a bad credential stops the plan rather than letting a
			// cluster resource fail half way through provisioning.
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether a credential Omni rejects fails the plan. Defaults to `true`; set it to `false` to read `valid` and decide in the configuration.",
			},
			"valid": omniComputedBool("Whether Omni accepted the service account."),
			"error": omniComputedString("Why Omni rejected the service account, empty when it accepted it."),
		},
	}
}

func (d *omniServiceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m omniServiceAccountModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	query.Set("endpoint", m.Endpoint.ValueString())
	query.Set("serviceAccountKey", m.ServiceAccountKey.ValueString())
	validateURL := d.client.Endpoint + "/omni/serviceaccount/validate?" + query.Encode()

	valid := true
	message := ""
	if err := doJSON(ctx, d.client, http.MethodGet, validateURL, nil, nil); err != nil {
		valid = false

		// The request URL carries the service account key as a query
		// parameter, so only what Omni said is reported - never the error
		// itself, which would put the credential into state and into the
		// diagnostics printed to the console.
		var statusErr *apiStatusError
		if errors.As(err, &statusErr) {
			message = statusErr.Body
		} else {
			message = "the validation request could not be sent"
		}

		if m.FailOnError.IsNull() || m.FailOnError.ValueBool() {
			omniAddError(&resp.Diagnostics, fmt.Errorf("the Omni service account was rejected (set fail_on_error = false to read the result instead): %s", message))
			return
		}
	}

	m.Valid = types.BoolValue(valid)
	m.Error = types.StringValue(message)
	m.ID = types.StringValue("portainer-omni-service-account")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
