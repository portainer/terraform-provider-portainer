package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The edge_settings block of portainer_custom_template: Portainer's
// CustomTemplateEdgeSettings, the Business Edition defaults an Edge stack
// deployed from an Edge template starts with. RelativePathSettings is
// flattened into the block, the way portainer_edge_stack names the same
// settings.

var customTemplateEdgeSettingsAttrTypes = map[string]attr.Type{
	"pre_pull_image":                      types.BoolType,
	"retry_deploy":                        types.BoolType,
	"retry_period":                        types.Int64Type,
	"private_registry_id":                 types.Int64Type,
	"relative_path":                       types.StringType,
	"per_device_configs_path":             types.StringType,
	"per_device_configs_match_type":       types.StringType,
	"per_device_configs_group_match_type": types.StringType,
	"stagger_config":                      types.ListType{ElemType: types.ObjectType{AttrTypes: edgeStaggerAttrTypes}},
}

type customTemplateEdgeSettingsModel struct {
	PrePullImage                   types.Bool   `tfsdk:"pre_pull_image"`
	RetryDeploy                    types.Bool   `tfsdk:"retry_deploy"`
	RetryPeriod                    types.Int64  `tfsdk:"retry_period"`
	PrivateRegistryID              types.Int64  `tfsdk:"private_registry_id"`
	RelativePath                   types.String `tfsdk:"relative_path"`
	PerDeviceConfigsPath           types.String `tfsdk:"per_device_configs_path"`
	PerDeviceConfigsMatchType      types.String `tfsdk:"per_device_configs_match_type"`
	PerDeviceConfigsGroupMatchType types.String `tfsdk:"per_device_configs_group_match_type"`
	StaggerConfig                  types.List   `tfsdk:"stagger_config"`
}

// customTemplateEdgeSettingsAPI is CustomTemplateEdgeSettings as Portainer
// reports it.
type customTemplateEdgeSettingsAPI struct {
	PrePullImage         bool            `json:"PrePullImage"`
	RetryDeploy          bool            `json:"RetryDeploy"`
	RetryPeriod          int64           `json:"RetryPeriod"`
	PrivateRegistryID    int64           `json:"PrivateRegistryId"`
	StaggerConfig        *edgeStaggerAPI `json:"StaggerConfig"`
	RelativePathSettings *struct {
		SupportRelativePath            bool   `json:"SupportRelativePath"`
		FilesystemPath                 string `json:"FilesystemPath"`
		SupportPerDeviceConfigs        bool   `json:"SupportPerDeviceConfigs"`
		PerDeviceConfigsPath           string `json:"PerDeviceConfigsPath"`
		PerDeviceConfigsMatchType      string `json:"PerDeviceConfigsMatchType"`
		PerDeviceConfigsGroupMatchType string `json:"PerDeviceConfigsGroupMatchType"`
	} `json:"RelativePathSettings"`
}

func customTemplateEdgeSettingsBlock() schema.ListNestedBlock {
	fileOrDir := func(requires string) []validator.String {
		return []validator.String{stringvalidator.OneOf("file", "dir"), stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(requires))}
	}
	return schema.ListNestedBlock{
		Description: "Defaults of an Edge stack deployed from this template (only for `edge_template = true`). Business Edition only; Community Edition ignores it. Only sent when the block is set.",
		Validators:  []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"pre_pull_image": schema.BoolAttribute{
					Optional:    true,
					Description: "Whether Edge agents pull the images before deploying.",
				},
				"retry_deploy": schema.BoolAttribute{
					Optional:    true,
					Description: "Whether the Edge agent retries a failed deployment.",
				},
				"retry_period": schema.Int64Attribute{
					Optional:    true,
					Validators:  []validator.Int64{int64validator.AtLeast(0)},
					Description: "How long, in seconds, the agent keeps retrying a failed deployment.",
				},
				"private_registry_id": schema.Int64Attribute{
					Optional:    true,
					Description: "ID of the registry the Edge stack pulls its images from.",
				},
				"relative_path": schema.StringAttribute{
					Optional:    true,
					Description: "Local filesystem path on the device for relative path volumes. Setting it turns relative paths on.",
				},
				"per_device_configs_path": schema.StringAttribute{
					Optional:    true,
					Validators:  []validator.String{stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("relative_path"))},
					Description: "Path within the repository holding per-device configurations. Setting it turns per-device configurations on. Requires `relative_path`.",
				},
				"per_device_configs_match_type": schema.StringAttribute{
					Optional:    true,
					Validators:  fileOrDir("per_device_configs_path"),
					Description: "How per-device configurations are matched to a device: `file` or `dir`.",
				},
				"per_device_configs_group_match_type": schema.StringAttribute{
					Optional:    true,
					Validators:  fileOrDir("per_device_configs_path"),
					Description: "How per-device configurations are matched to an Edge group: `file` or `dir`.",
				},
			},
			Blocks: map[string]schema.Block{
				"stagger_config": edgeStaggerConfigBlock(),
			},
		},
	}
}

// customTemplateEdgeSettingsPayload returns the EdgeSettings payload, or nil
// (not sent) when the block is not set.
func customTemplateEdgeSettingsPayload(ctx context.Context, l types.List, diags *diag.Diagnostics) map[string]interface{} {
	if l.IsNull() || l.IsUnknown() || len(l.Elements()) == 0 {
		return nil
	}
	var blocks []customTemplateEdgeSettingsModel
	diags.Append(l.ElementsAs(ctx, &blocks, false)...)
	if len(blocks) == 0 {
		return nil
	}
	b := blocks[0]
	payload := map[string]interface{}{
		"PrePullImage":      b.PrePullImage.ValueBool(),
		"RetryDeploy":       b.RetryDeploy.ValueBool(),
		"RetryPeriod":       b.RetryPeriod.ValueInt64(),
		"PrivateRegistryId": b.PrivateRegistryID.ValueInt64(),
	}
	if p := b.RelativePath.ValueString(); p != "" {
		rel := map[string]interface{}{"SupportRelativePath": true, "FilesystemPath": p}
		if dp := b.PerDeviceConfigsPath.ValueString(); dp != "" {
			rel["SupportPerDeviceConfigs"] = true
			rel["PerDeviceConfigsPath"] = dp
			if v := b.PerDeviceConfigsMatchType.ValueString(); v != "" {
				rel["PerDeviceConfigsMatchType"] = v
			}
			if v := b.PerDeviceConfigsGroupMatchType.ValueString(); v != "" {
				rel["PerDeviceConfigsGroupMatchType"] = v
			}
		}
		payload["RelativePathSettings"] = rel
	}
	if stagger := edgeStaggerPayload(ctx, b.StaggerConfig, diags); stagger != nil {
		payload["StaggerConfig"] = stagger
	}
	return payload
}

// customTemplateEdgeSettingsFromAPI reads the edge_settings block back. As
// for stagger_config, only a block the configuration manages is read, and a
// server that reports none (Community Edition) leaves it as it was; unset
// attributes stay null while Portainer reports their zero value.
func customTemplateEdgeSettingsFromAPI(ctx context.Context, prior types.List, api *customTemplateEdgeSettingsAPI, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: customTemplateEdgeSettingsAttrTypes}
	if prior.IsNull() || prior.IsUnknown() || len(prior.Elements()) == 0 {
		return types.ListValueMust(objType, []attr.Value{})
	}
	if api == nil {
		return prior
	}
	var blocks []customTemplateEdgeSettingsModel
	diags.Append(prior.ElementsAs(ctx, &blocks, false)...)
	if len(blocks) == 0 {
		return prior
	}
	p := blocks[0]
	boolFromAPI := func(prior types.Bool, v bool) types.Bool {
		if !v && prior.IsNull() {
			return types.BoolNull()
		}
		return types.BoolValue(v)
	}
	relPath, devPath, devMatch, groupMatch := "", "", "", ""
	if rel := api.RelativePathSettings; rel != nil {
		if rel.SupportRelativePath {
			relPath = rel.FilesystemPath
		}
		if rel.SupportPerDeviceConfigs {
			devPath, devMatch, groupMatch = rel.PerDeviceConfigsPath, rel.PerDeviceConfigsMatchType, rel.PerDeviceConfigsGroupMatchType
		}
	}
	obj, d := types.ObjectValue(customTemplateEdgeSettingsAttrTypes, map[string]attr.Value{
		"pre_pull_image":                      boolFromAPI(p.PrePullImage, api.PrePullImage),
		"retry_deploy":                        boolFromAPI(p.RetryDeploy, api.RetryDeploy),
		"retry_period":                        edgeInt64FromAPI(p.RetryPeriod, api.RetryPeriod),
		"private_registry_id":                 edgeInt64FromAPI(p.PrivateRegistryID, api.PrivateRegistryID),
		"relative_path":                       edgeStringFromAPI(p.RelativePath, relPath),
		"per_device_configs_path":             edgeStringFromAPI(p.PerDeviceConfigsPath, devPath),
		"per_device_configs_match_type":       edgeStringFromAPI(p.PerDeviceConfigsMatchType, devMatch),
		"per_device_configs_group_match_type": edgeStringFromAPI(p.PerDeviceConfigsGroupMatchType, groupMatch),
		"stagger_config":                      edgeStaggerFromAPI(ctx, p.StaggerConfig, api.StaggerConfig, diags),
	})
	diags.Append(d...)
	return types.ListValueMust(objType, []attr.Value{obj})
}
