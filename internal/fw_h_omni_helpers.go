package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Helpers of the Omni resources and data sources (plugin framework).

// omniAddError reports an error the way diag.FromErr did in the SDKv2
// version: the error text is the summary.
func omniAddError(diags *diag.Diagnostics, err error) {
	diags.AddError(err.Error(), "")
}

// omniSDKValue turns a Terraform value into the shape SDKv2's d.Get returned:
// objects become map[string]interface{}, lists []interface{}, numbers int, and
// null or unknown primitives their zero value. The payload builders below
// were written against that shape, so the requests stay exactly the same.
func omniSDKValue(v tftypes.Value) interface{} {
	typ := v.Type()
	known := v.IsKnown() && !v.IsNull()
	switch {
	case typ.Is(tftypes.String):
		var s string
		if known {
			_ = v.As(&s)
		}
		return s
	case typ.Is(tftypes.Number):
		if !known {
			return 0
		}
		var f big.Float
		_ = v.As(&f)
		n, _ := f.Int64()
		return int(n)
	case typ.Is(tftypes.Bool):
		var b bool
		if known {
			_ = v.As(&b)
		}
		return b
	case typ.Is(tftypes.List{}), typ.Is(tftypes.Set{}), typ.Is(tftypes.Tuple{}):
		out := []interface{}{}
		if !known {
			return out
		}
		var elems []tftypes.Value
		_ = v.As(&elems)
		for _, e := range elems {
			out = append(out, omniSDKValue(e))
		}
		return out
	case typ.Is(tftypes.Map{}), typ.Is(tftypes.Object{}):
		out := map[string]interface{}{}
		if !known {
			return out
		}
		var elems map[string]tftypes.Value
		_ = v.As(&elems)
		for k, e := range elems {
			out[k] = omniSDKValue(e)
		}
		return out
	}
	return nil
}

// omniSDKZeroIsNull is the plan modifier of the optional attributes inside
// portainer_omni_cluster's machine blocks. Those blocks are never read back,
// and SDKv2 (v3.0.0) stored "" / 0 / false / [] for an argument the
// configuration left out of a block, where the framework keeps null. The
// attributes are optional+computed so that, for an unset argument, the plan
// keeps a zero value already in state instead of planning "" -> null; any
// other state value plans null as usual. A configured value is never touched.
type omniSDKZeroIsNull struct{}

func (omniSDKZeroIsNull) Description(context.Context) string {
	return "Treats a zero value stored by SDKv2 as equal to an unset argument."
}

func (m omniSDKZeroIsNull) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func omniIsZero(v attr.Value) bool {
	if v == nil || v.IsNull() {
		return true
	}
	if v.IsUnknown() {
		return false
	}
	switch t := v.(type) {
	case types.String:
		return t.ValueString() == ""
	case types.Int64:
		return t.ValueInt64() == 0
	case types.Bool:
		return !t.ValueBool()
	case types.List:
		return len(t.Elements()) == 0
	}
	return false
}

func (omniSDKZeroIsNull) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if omniIsZero(req.StateValue) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.StringNull()
}

func (omniSDKZeroIsNull) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if omniIsZero(req.StateValue) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.Int64Null()
}

func (omniSDKZeroIsNull) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if omniIsZero(req.StateValue) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.BoolNull()
}

func (omniSDKZeroIsNull) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	if omniIsZero(req.StateValue) {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.ListNull(types.StringType)
}

// omniJSONObjectValidator is validateJSONObject for the framework: an empty
// value is accepted, anything else must decode as a JSON object.
type omniJSONObjectValidator struct{}

func (omniJSONObjectValidator) Description(context.Context) string {
	return "value must be a JSON object"
}

func (v omniJSONObjectValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (omniJSONObjectValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() == "" {
		return
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(req.ConfigValue.ValueString()), &decoded); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid JSON object",
			fmt.Sprintf("%s must be a JSON object (use jsonencode): %s", req.Path, err))
	}
}

// omniMachinesFromConfig turns a machine block list into the payload Omni
// expects. Every optional field is omitted when empty so a machine definition
// never sends a zero value Omni would read as a real setting.
func omniMachinesFromConfig(raw interface{}) ([]map[string]interface{}, error) {
	list, _ := raw.([]interface{})
	machines := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		machine := map[string]interface{}{"name": block["name"]}
		for field, key := range map[string]string{
			"hostname": "hostname",
			"kind":     "kind",
		} {
			if v, ok := block[field].(string); ok && v != "" {
				machine[key] = v
			}
		}
		if v, ok := block["install_disk"].(string); ok && v != "" {
			machine["install"] = map[string]interface{}{"disk": v}
		}
		if v, ok := block["nameservers"].([]interface{}); ok && len(v) > 0 {
			machine["nameservers"] = v
		}
		if v, ok := block["system_disk_size"].(int); ok && v != 0 {
			machine["systemDiskSize"] = v
		}

		if disks, ok := block["user_disk"].([]interface{}); ok && len(disks) > 0 {
			if disk, ok := disks[0].(map[string]interface{}); ok {
				machine["userDisk"] = map[string]interface{}{
					"volumeName": disk["volume_name"],
					// Zero is meaningful here: Omni reads it as "grow into
					// whatever space is left", so it is always sent.
					"size": disk["size"],
				}
			}
		}

		if interfaces, ok := block["interface"].([]interface{}); ok && len(interfaces) > 0 {
			encoded := make([]map[string]interface{}, 0, len(interfaces))
			for _, raw := range interfaces {
				iface, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				entry := map[string]interface{}{"interface": iface["interface"]}
				if v, ok := iface["dhcp"].(bool); ok && v {
					entry["dhcp"] = v
				}
				if v, ok := iface["addresses"].([]interface{}); ok && len(v) > 0 {
					entry["addresses"] = v
				}
				if routes, ok := iface["route"].([]interface{}); ok && len(routes) > 0 {
					encodedRoutes := make([]map[string]interface{}, 0, len(routes))
					for _, raw := range routes {
						route, ok := raw.(map[string]interface{})
						if !ok {
							continue
						}
						encodedRoutes = append(encodedRoutes, map[string]interface{}{
							"network": route["network"],
							"gateway": route["gateway"],
						})
					}
					entry["routes"] = encodedRoutes
				}
				encoded = append(encoded, entry)
			}
			machine["interfaces"] = encoded
		}

		patches, err := omniMachinePatches(block["patch"])
		if err != nil {
			return nil, fmt.Errorf("machine %v: %w", block["name"], err)
		}
		if len(patches) > 0 {
			machine["patches"] = patches
		}

		machines = append(machines, machine)
	}
	return machines, nil
}

// omniMachinePatches decodes the inline patch bodies, which are JSON in the
// configuration and objects on the wire.
func omniMachinePatches(raw interface{}) ([]map[string]interface{}, error) {
	list, _ := raw.([]interface{})
	patches := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		patch := map[string]interface{}{}
		if v, ok := block["id_override"].(string); ok && v != "" {
			patch["idOverride"] = v
		}
		if v, ok := block["inline"].(string); ok && v != "" {
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse the patch body as a JSON object: %w", err)
			}
			patch["inline"] = decoded
		}
		if len(patch) > 0 {
			patches = append(patches, patch)
		}
	}
	return patches, nil
}

// omniControlPlane and omniWorker build the two machine groups. They return
// false when the block is absent, which is how the payload leaves it out.
func omniControlPlane(raw interface{}) (map[string]interface{}, bool, error) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false, nil
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	machines, err := omniMachinesFromConfig(block["machine"])
	if err != nil {
		return nil, false, err
	}
	group := map[string]interface{}{"machines": machines}
	if v, ok := block["kind"].(string); ok && v != "" {
		group["kind"] = v
	}
	return group, true, nil
}

func omniWorker(raw interface{}) (map[string]interface{}, bool, error) {
	group, ok, err := omniControlPlane(raw)
	if !ok || err != nil {
		return nil, ok, err
	}
	list, _ := raw.([]interface{})
	if block, ok := list[0].(map[string]interface{}); ok {
		if v, ok := block["name"].(string); ok && v != "" {
			group["name"] = v
		}
	}
	return group, true, nil
}

// omniClusterSpec builds the cluster half of the payload, which create,
// update and validate all send. d is the resource in omniSDKValue's shape.
func omniClusterSpec(d map[string]interface{}) (map[string]interface{}, error) {
	cluster := map[string]interface{}{"name": d["name"]}
	if v, _ := d["kind"].(string); v != "" {
		cluster["kind"] = v
	}
	if v, _ := d["kubernetes_version"].(string); v != "" {
		cluster["kubernetes"] = map[string]interface{}{"version": v}
	}
	if v, _ := d["talos_version"].(string); v != "" {
		cluster["talos"] = map[string]interface{}{"version": v}
	}
	if labels, ok := d["labels"].(map[string]interface{}); ok && len(labels) > 0 {
		cluster["labels"] = labels
	}

	list, _ := d["cluster_patch"].([]interface{})
	patches := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		patch := map[string]interface{}{}
		if v, ok := block["id_override"].(string); ok && v != "" {
			patch["idOverride"] = v
		}
		if v, ok := block["annotation_name"].(string); ok && v != "" {
			patch["annotations"] = map[string]interface{}{"name": v}
		}
		if v, ok := block["inline"].(string); ok && v != "" {
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(v), &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse a cluster patch body as a JSON object: %w", err)
			}
			patch["inline"] = decoded
		}
		if len(patch) > 0 {
			patches = append(patches, patch)
		}
	}
	if len(patches) > 0 {
		cluster["patches"] = patches
	}

	return cluster, nil
}

// omniClusterID and omniClusterParts keep the resource ID - credential and
// cluster name - in one place, since every call needs both. IDs from before
// v3.0.0 use "/" instead of ":" and are still accepted.
func omniClusterID(credentialID int, name string) string {
	return fmt.Sprintf("%d:%s", credentialID, name)
}

func omniClusterParts(id string) (int, string, error) {
	parts, err := importIDPartsCompat(id, "/", 2, "<credential id>:<cluster name>")
	if err != nil {
		return 0, "", err
	}
	credential, name := parts[0], parts[1]
	var credentialID int
	if _, err := fmt.Sscanf(credential, "%d", &credentialID); err != nil {
		return 0, "", fmt.Errorf("the credential part of the resource ID must be a number, got %q", credential)
	}
	return credentialID, name, nil
}

// omniMachineNames lists the machine names in a group block, which is what the
// update endpoint removes machines by.
func omniMachineNames(raw interface{}) []string {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil
	}
	machines, _ := block["machine"].([]interface{})
	names := make([]string, 0, len(machines))
	for _, item := range machines {
		if machine, ok := item.(map[string]interface{}); ok {
			if name, ok := machine["name"].(string); ok && name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// omniGroupDelta works out which machines to add and which to remove. The
// update endpoint is imperative - it takes machines to add and names to remove
// - so the declarative machine list has to be turned into that difference.
func omniGroupDelta(before, after interface{}) (added []interface{}, removed []string) {
	beforeNames := map[string]bool{}
	for _, name := range omniMachineNames(before) {
		beforeNames[name] = true
	}
	afterNames := map[string]bool{}
	for _, name := range omniMachineNames(after) {
		afterNames[name] = true
	}

	for _, name := range omniMachineNames(before) {
		if !afterNames[name] {
			removed = append(removed, name)
		}
	}

	if list, ok := after.([]interface{}); ok && len(list) > 0 {
		if block, ok := list[0].(map[string]interface{}); ok {
			machines, _ := block["machine"].([]interface{})
			for _, item := range machines {
				machine, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if name, ok := machine["name"].(string); ok && !beforeNames[name] {
					added = append(added, machine)
				}
			}
		}
	}
	return added, removed
}

// omniGroupFromMachines wraps a bare machine list back into the group shape
// the update payload takes.
func omniGroupFromMachines(source interface{}, machines []interface{}) (map[string]interface{}, error) {
	encoded, err := omniMachinesFromConfig(machines)
	if err != nil {
		return nil, err
	}
	group := map[string]interface{}{"machines": encoded}
	if list, ok := source.([]interface{}); ok && len(list) > 0 {
		if block, ok := list[0].(map[string]interface{}); ok {
			if v, ok := block["kind"].(string); ok && v != "" {
				group["kind"] = v
			}
			if v, ok := block["name"].(string); ok && v != "" {
				group["name"] = v
			}
		}
	}
	return group, nil
}

// omniMachineStatus is the machine shape Portainer returns from both the list
// and the inspect endpoint. The list data source surfaces the summary fields;
// the inspect one also unpacks the hardware and network detail.
type omniMachineStatus struct {
	MachineName string            `json:"machineName"`
	Labels      map[string]string `json:"labels"`
	Spec        struct {
		Cluster           string `json:"cluster"`
		Connected         bool   `json:"connected"`
		Maintenance       bool   `json:"maintenance"`
		ManagementAddress string `json:"management_address"`
		PowerState        string `json:"power_state"`
		Role              string `json:"role"`
		TalosVersion      string `json:"talos_version"`
		LastError         string `json:"last_error"`
		Hardware          struct {
			Arch         string `json:"arch"`
			BlockDevices []struct {
				LinuxName  string `json:"linux_name"`
				Model      string `json:"model"`
				Size       int64  `json:"size"`
				Type       string `json:"type"`
				SystemDisk bool   `json:"system_disk"`
			} `json:"blockdevices"`
			MemoryModules []struct {
				Description string `json:"description"`
				SizeMB      int    `json:"size_mb"`
			} `json:"memory_modules"`
			Processors []struct {
				Description  string `json:"description"`
				Manufacturer string `json:"manufacturer"`
				CoreCount    int    `json:"core_count"`
				ThreadCount  int    `json:"thread_count"`
				Frequency    int    `json:"frequency"`
			} `json:"processors"`
		} `json:"hardware"`
		Network struct {
			Hostname        string   `json:"hostname"`
			DomainName      string   `json:"domainname"`
			Addresses       []string `json:"addresses"`
			DefaultGateways []string `json:"default_gateways"`
			NetworkLinks    []struct {
				LinuxName       string `json:"linux_name"`
				HardwareAddress string `json:"hardware_address"`
				LinkUp          bool   `json:"link_up"`
				SpeedMbps       int    `json:"speed_mbps"`
			} `json:"network_links"`
		} `json:"network"`
		PlatformMetadata struct {
			Platform     string `json:"platform"`
			Hostname     string `json:"hostname"`
			InstanceID   string `json:"instance_id"`
			InstanceType string `json:"instance_type"`
			Region       string `json:"region"`
			Zone         string `json:"zone"`
		} `json:"platform_metadata"`
	} `json:"spec"`
}

// omniStringList is a list(string) that, like SDKv2, holds an empty list for
// a missing API value.
func omniStringList(values []string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}
