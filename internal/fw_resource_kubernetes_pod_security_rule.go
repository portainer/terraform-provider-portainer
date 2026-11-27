package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesPodSecurityRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesPodSecurityRuleResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesPodSecurityRuleResource)(nil)
)

// kubernetesPodSecurityRuleResource manages the OPA-backed pod security rule
// of a Kubernetes environment.
//
// Portainer always has a rule for a cluster - its own UI creates a default one
// on first view - so this resource adopts and configures that rule rather than
// creating one. Destroying it switches the rule off rather than deleting it,
// which is the only "absent" state the endpoint has.
func newKubernetesPodSecurityRuleResource() resource.Resource {
	return &kubernetesPodSecurityRuleResource{}
}

type kubernetesPodSecurityRuleResource struct{ k8sfwBase }

func (r *kubernetesPodSecurityRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_pod_security_rule"
}

// podSecuritySwitch is one of the plain switches of the rule.
func podSecuritySwitch(def bool, description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Default:       booldefault.StaticBool(def),
		Description:   description,
		PlanModifiers: []planmodifier.Bool{k8sfwKeepNull{}},
	}
}

// podSecuritySectionEnabled is the "enabled" switch every section has.
func podSecuritySectionEnabled() schema.BoolAttribute {
	return podSecuritySwitch(true, "Whether this section is enforced.")
}

// podSecuritySectionBlock is a MaxItems-1 section of the rule.
func podSecuritySectionBlock(description string, attrs map[string]schema.Attribute, blocks map[string]schema.Block) schema.ListNestedBlock {
	attrs["enabled"] = podSecuritySectionEnabled()
	return schema.ListNestedBlock{
		Description:  description,
		Validators:   []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{Attributes: attrs, Blocks: blocks},
	}
}

func podSecurityStrings(description string) schema.ListAttribute {
	return schema.ListAttribute{ElementType: types.StringType, Optional: true, Description: description}
}

// podSecurityStrategyBlock is a run-as strategy with its identifier ranges.
func podSecurityStrategyBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		Validators:  []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"type": schema.StringAttribute{
					Optional:    true,
					Description: "Strategy, for example `MustRunAs` or `RunAsAny`.",
				},
			},
			Blocks: map[string]schema.Block{
				"id_range": schema.ListNestedBlock{
					Description: "Permitted identifier ranges. Repeatable.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"min": schema.Int64Attribute{Required: true, Description: "Lowest identifier in the range."},
							"max": schema.Int64Attribute{Required: true, Description: "Highest identifier in the range."},
						},
					},
				},
			},
		},
	}
}

func (r *kubernetesPodSecurityRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the environment the rule applies to.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Kubernetes environment the rule applies to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"enabled":                    podSecuritySwitch(true, "Whether the pod security rule is enforced at all. With this off, the individual sections below are stored but do nothing."),
			"privileged_containers":      podSecuritySwitch(false, "Whether to block privileged containers."),
			"allow_privilege_escalation": podSecuritySwitch(false, "Whether to block containers that may escalate their privileges."),
			"host_namespaces":            podSecuritySwitch(false, "Whether to block pods that join the host's namespaces."),
			"read_only_root_filesystem":  podSecuritySwitch(false, "Whether to require a read-only root filesystem."),
			"restrict_default_namespace": podSecuritySwitch(false, "Whether to stop workloads being deployed into the `default` namespace."),
			"restrict_secrets":           podSecuritySwitch(false, "Whether to restrict access to secrets."),
		},
		Blocks: map[string]schema.Block{
			"allow_proc_mount": podSecuritySectionBlock("Which proc mount types are permitted.", map[string]schema.Attribute{
				"proc_mount_type": schema.StringAttribute{
					Optional:    true,
					Description: "The proc mount type to permit, for example `Default` or `Unmasked`.",
				},
			}, nil),
			"allow_flex_volumes": podSecuritySectionBlock("Which FlexVolume drivers are permitted.", map[string]schema.Attribute{
				"allowed_volumes": podSecurityStrings("FlexVolume drivers a pod may use."),
			}, nil),
			"app_armor": podSecuritySectionBlock("Which AppArmor profiles are permitted.", map[string]schema.Attribute{
				"types": podSecurityStrings("AppArmor profiles a pod may use."),
			}, nil),
			"capabilities": podSecuritySectionBlock("Which Linux capabilities a container may hold.", map[string]schema.Attribute{
				"allowed":       podSecurityStrings("Capabilities a container may add."),
				"required_drop": podSecurityStrings("Capabilities every container has to drop."),
			}, nil),
			"forbidden_sysctls": podSecuritySectionBlock("Which sysctls a pod may not set.", map[string]schema.Attribute{
				// Portainer stores this list under a field named
				// requiredDropCapabilities, which is a misnomer in its own
				// schema rather than something to copy here.
				"sysctls": podSecurityStrings("Sysctls a pod may not set."),
			}, nil),
			"host_filesystem": podSecuritySectionBlock("Which host paths a pod may mount.", map[string]schema.Attribute{}, map[string]schema.Block{
				"allowed_path": schema.ListNestedBlock{
					Description: "Host paths a pod may mount. Repeatable.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"path_prefix": schema.StringAttribute{Required: true, Description: "Path prefix that may be mounted."},
							"readonly":    podSecuritySwitch(false, "Whether the mount has to be read-only."),
						},
					},
				},
			}),
			"host_ports": podSecuritySectionBlock("Which host ports a pod may bind, and whether host networking is allowed at all.", map[string]schema.Attribute{
				"host_network": podSecuritySwitch(false, "Whether a pod may use the host's network namespace."),
				"min":          schema.Int64Attribute{Optional: true, Description: "Lowest host port a pod may bind."},
				"max":          schema.Int64Attribute{Optional: true, Description: "Highest host port a pod may bind."},
			}, nil),
			"sec_comp": podSecuritySectionBlock("Which seccomp profiles are permitted.", map[string]schema.Attribute{
				"types": podSecurityStrings("Seccomp profiles a pod may use."),
			}, nil),
			"selinux": podSecuritySectionBlock("Which SELinux contexts are permitted.", map[string]schema.Attribute{}, map[string]schema.Block{
				"allowed_context": schema.ListNestedBlock{
					Description: "SELinux contexts a pod may run under. Repeatable.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"user":  schema.StringAttribute{Optional: true, Description: "SELinux user."},
							"role":  schema.StringAttribute{Optional: true, Description: "SELinux role."},
							"type":  schema.StringAttribute{Optional: true, Description: "SELinux type."},
							"level": schema.StringAttribute{Optional: true, Description: "SELinux level."},
						},
					},
				},
			}),
			"volume_types": podSecuritySectionBlock("Which volume types a pod may use.", map[string]schema.Attribute{
				"allowed_types": podSecurityStrings("Volume types a pod may use, for example `configMap` or `emptyDir`."),
			}, nil),
			"users": podSecuritySectionBlock("Which user and group identities a pod may run as.", map[string]schema.Attribute{}, map[string]schema.Block{
				"run_as_user":         podSecurityStrategyBlock("Strategy and ranges for the user a container runs as."),
				"run_as_group":        podSecurityStrategyBlock("Strategy and ranges for the group a container runs as."),
				"fs_groups":           podSecurityStrategyBlock("Strategy and ranges for the pod's filesystem groups."),
				"supplemental_groups": podSecurityStrategyBlock("Strategy and ranges for the pod's supplemental groups."),
			}),
		},
	}
}

// podSecurityBlock reads a MaxItems-1 block, returning the block's contents
// and whether it was present at all.
func podSecurityBlock(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	list, _ := data[key].([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	return block, ok
}

// podSecuritySection builds the {"enabled": ...} object every section shares.
func podSecuritySection(block map[string]interface{}) map[string]interface{} {
	enabled, _ := block["enabled"].(bool)
	return map[string]interface{}{"enabled": enabled}
}

// podSecurityStrategy encodes a run-as strategy with its identifier ranges.
func podSecurityStrategy(raw interface{}) (map[string]interface{}, bool) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false
	}
	strategy := map[string]interface{}{}
	if v, ok := block["type"].(string); ok && v != "" {
		strategy["type"] = v
	}
	ranges, _ := block["id_range"].([]interface{})
	encoded := make([]map[string]interface{}, 0, len(ranges))
	for _, item := range ranges {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		encoded = append(encoded, map[string]interface{}{
			"min": entry["min"], "max": entry["max"],
		})
	}
	if len(encoded) > 0 {
		strategy["idrange"] = encoded
	}
	return strategy, true
}

func podSecurityRulePayload(data map[string]interface{}) map[string]interface{} {
	endpointID, _ := data["endpoint_id"].(int)
	enabled, _ := data["enabled"].(bool)
	payload := map[string]interface{}{
		"endPointID": endpointID,
		"enabled":    enabled,
	}

	// The plain switches, which Portainer stores as one-field objects.
	for field, key := range map[string]string{
		"privileged_containers":      "privilegedContainers",
		"allow_privilege_escalation": "allowPrivilegeEscalation",
		"host_namespaces":            "hostNamespaces",
		"read_only_root_filesystem":  "readOnlyRootFileSystem",
		"restrict_default_namespace": "restrictDefaultNamespace",
		"restrict_secrets":           "restrictSecrets",
	} {
		on, _ := data[field].(bool)
		payload[key] = map[string]interface{}{"enabled": on}
	}

	if block, ok := podSecurityBlock(data, "allow_proc_mount"); ok {
		section := podSecuritySection(block)
		if v, ok := block["proc_mount_type"].(string); ok && v != "" {
			section["procMountType"] = v
		}
		payload["allowProcMount"] = section
	}
	if block, ok := podSecurityBlock(data, "allow_flex_volumes"); ok {
		section := podSecuritySection(block)
		section["allowedVolumes"] = block["allowed_volumes"]
		payload["allowFlexVolumes"] = section
	}
	if block, ok := podSecurityBlock(data, "app_armor"); ok {
		section := podSecuritySection(block)
		section["AppArmorType"] = block["types"]
		payload["appArmor"] = section
	}
	if block, ok := podSecurityBlock(data, "capabilities"); ok {
		section := podSecuritySection(block)
		section["allowedCapabilities"] = block["allowed"]
		section["requiredDropCapabilities"] = block["required_drop"]
		payload["capabilities"] = section
	}
	if block, ok := podSecurityBlock(data, "forbidden_sysctls"); ok {
		section := podSecuritySection(block)
		// Portainer stores the sysctl list under requiredDropCapabilities,
		// which is a misnomer in its own schema.
		section["requiredDropCapabilities"] = block["sysctls"]
		payload["forbiddenSysctlsList"] = section
	}
	if block, ok := podSecurityBlock(data, "host_filesystem"); ok {
		section := podSecuritySection(block)
		paths, _ := block["allowed_path"].([]interface{})
		encoded := make([]map[string]interface{}, 0, len(paths))
		for _, item := range paths {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			encoded = append(encoded, map[string]interface{}{
				"pathPrefix": entry["path_prefix"], "readonly": entry["readonly"],
			})
		}
		section["allowedPaths"] = encoded
		payload["hostFilesystem"] = section
	}
	if block, ok := podSecurityBlock(data, "host_ports"); ok {
		section := podSecuritySection(block)
		section["hostNetwork"] = block["host_network"]
		section["min"] = block["min"]
		section["max"] = block["max"]
		payload["hostPorts"] = section
	}
	if block, ok := podSecurityBlock(data, "sec_comp"); ok {
		section := podSecuritySection(block)
		section["secCompType"] = block["types"]
		payload["secComp"] = section
	}
	if block, ok := podSecurityBlock(data, "selinux"); ok {
		section := podSecuritySection(block)
		contexts, _ := block["allowed_context"].([]interface{})
		encoded := make([]map[string]interface{}, 0, len(contexts))
		for _, item := range contexts {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			encoded = append(encoded, map[string]interface{}{
				"user": entry["user"], "role": entry["role"],
				"type": entry["type"], "level": entry["level"],
			})
		}
		section["allowedCapabilities"] = encoded
		payload["selinux"] = section
	}
	if block, ok := podSecurityBlock(data, "volume_types"); ok {
		section := podSecuritySection(block)
		section["allowedTypes"] = block["allowed_types"]
		payload["volumeTypes"] = section
	}
	if block, ok := podSecurityBlock(data, "users"); ok {
		section := podSecuritySection(block)
		for field, key := range map[string]string{
			"run_as_user":         "runAsUser",
			"run_as_group":        "runAsGroup",
			"fs_groups":           "fsGroups",
			"supplemental_groups": "supplementalGroups",
		} {
			if strategy, ok := podSecurityStrategy(block[field]); ok {
				section[key] = strategy
			}
		}
		payload["users"] = section
	}

	return payload
}

func (r *kubernetesPodSecurityRuleResource) write(ctx context.Context, data map[string]interface{}, diags *diag.Diagnostics) {
	endpointID, _ := data["endpoint_id"].(int)

	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodPut, ruleURL, podSecurityRulePayload(data), nil); err != nil {
		diags.AddError("Failed to update the pod security rule", fmt.Errorf("failed to update the pod security rule of environment %d: %w", endpointID, err).Error())
		return
	}

	data["id"] = fmt.Sprintf("%d", endpointID)
	if _, err := r.read(ctx, data); err != nil {
		diags.AddError("Failed to read the pod security rule", err.Error())
	}
}

// read refreshes the switches of data, reporting false when the rule is gone.
//
// Only the switches are read back. The individual sections are lists of
// permitted values that Portainer normalises and reorders, so reading them
// into state would turn a stable configuration into a churning plan; what
// they contain is driven entirely by the configuration instead.
func (r *kubernetesPodSecurityRuleResource) read(ctx context.Context, data map[string]interface{}) (bool, error) {
	id, _ := data["id"].(string)
	var endpointID int
	if _, err := fmt.Sscanf(id, "%d", &endpointID); err != nil {
		return false, fmt.Errorf("the resource ID must be an environment identifier, got %q", id)
	}

	var rule struct {
		Enabled              bool `json:"enabled"`
		EndpointID           int  `json:"endPointID"`
		PrivilegedContainers struct {
			Enabled bool `json:"enabled"`
		} `json:"privilegedContainers"`
		AllowPrivilegeEscalation struct {
			Enabled bool `json:"enabled"`
		} `json:"allowPrivilegeEscalation"`
		HostNamespaces struct {
			Enabled bool `json:"enabled"`
		} `json:"hostNamespaces"`
		ReadOnlyRootFileSystem struct {
			Enabled bool `json:"enabled"`
		} `json:"readOnlyRootFileSystem"`
		RestrictDefaultNamespace struct {
			Enabled bool `json:"enabled"`
		} `json:"restrictDefaultNamespace"`
		RestrictSecrets struct {
			Enabled bool `json:"enabled"`
		} `json:"restrictSecrets"`
	}

	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodGet, ruleURL, nil, &rule); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read the pod security rule of environment %d: %w", endpointID, err)
	}

	data["endpoint_id"] = endpointID
	data["enabled"] = rule.Enabled
	data["privileged_containers"] = rule.PrivilegedContainers.Enabled
	data["allow_privilege_escalation"] = rule.AllowPrivilegeEscalation.Enabled
	data["host_namespaces"] = rule.HostNamespaces.Enabled
	data["read_only_root_filesystem"] = rule.ReadOnlyRootFileSystem.Enabled
	data["restrict_default_namespace"] = rule.RestrictDefaultNamespace.Enabled
	data["restrict_secrets"] = rule.RestrictSecrets.Enabled
	return true, nil
}

func (r *kubernetesPodSecurityRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesPodSecurityRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the pod security rule", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesPodSecurityRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete switches the rule off. Portainer has no endpoint to remove a rule -
// a cluster always has one - so that is the only "absent" state available,
// and leaving an enforced policy behind for a rule Terraform no longer
// manages would be worse.
func (r *kubernetesPodSecurityRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var endpointID int
	if _, err := fmt.Sscanf(id.ValueString(), "%d", &endpointID); err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("the resource ID must be an environment identifier, got %q", id.ValueString()))
		return
	}

	payload := map[string]interface{}{"endPointID": endpointID, "enabled": false}
	ruleURL := fmt.Sprintf("%s/kubernetes/%d/opa", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodPut, ruleURL, payload, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to disable the pod security rule", fmt.Errorf("failed to disable the pod security rule of environment %d: %w", endpointID, err).Error())
	}
}

// ImportState takes the environment ID.
func (r *kubernetesPodSecurityRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
