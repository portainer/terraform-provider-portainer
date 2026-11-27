package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// portainer_docker_plugin installs a Docker plugin on an environment. Every
// attribute but the informational description of a setting forces a new
// plugin.

var newDockerPluginResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_plugin",
	schema:     dockerPluginSchema,
	create:     dockerPluginCreate,
	read:       dockerPluginRead,
	update:     dockerPluginUpdate,
	delete:     dockerPluginDelete,
	importer:   dockerPluginImport,
	identity:   dockerPluginIdentity,
})

func dockerPluginSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"enable":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether to enable the Docker plugin immediately after installation.", PlanModifiers: []planmodifier.Bool{dkReplaceBool()}},
			"endpoint_id":   schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker host) where the plugin is installed.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"name":          schema.StringAttribute{Optional: true, Computed: true, Description: "Local alias name for the installed Docker plugin. Defaults to `remote`.", PlanModifiers: []planmodifier.String{dkReplaceString(), stringplanmodifier.UseStateForUnknown()}},
			"registry_auth": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("e30="), Description: "Base64-encoded registry authentication payload used to pull the plugin from a private registry.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"remote":        schema.StringAttribute{Required: true, Description: "Remote source URL or registry reference of the Docker plugin to pull.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
		},
		Blocks: map[string]schema.Block{
			"settings": schema.ListNestedBlock{
				Description:   "Configuration settings applied to the Docker plugin during installation.",
				PlanModifiers: []planmodifier.List{dockerPluginSettingsPlanModifier{}},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{Optional: true, Description: "Human-readable description of the plugin setting. Informational: Docker ignores it, so a change does not replace an existing plugin."},
						"name":        schema.StringAttribute{Required: true, Description: "Name of the Docker plugin setting."},
						"value":       schema.ListAttribute{ElementType: types.StringType, Required: true, Description: "List of values assigned to the plugin setting."},
					},
				},
			},
		},
	}
}

func dockerPluginCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	remote := dkString(d, "remote")
	name := dkString(d, "name")
	enable := dkBool(d, "enable")
	auth := dkString(d, "registry_auth")

	query := url.Values{"remote": {remote}}
	if name != "" {
		query.Set("name", name)
	}

	var settings []interface{}
	if v, ok := d.GetOk("settings"); ok {
		for _, s := range v.([]interface{}) {
			item := s.(map[string]interface{})
			entry := map[string]interface{}{
				"Name":  item["name"].(string),
				"Value": item["value"].([]interface{}),
			}
			if desc, ok := item["description"]; ok && desc.(string) != "" {
				entry["Description"] = desc.(string)
			}
			settings = append(settings, entry)
		}
	}

	headers := map[string]string{
		"X-Registry-Auth": auth, // base64 encoded '{}'
	}

	path := fmt.Sprintf("/endpoints/%d/docker/plugins/pull?%s", endpointID, query.Encode())
	resp, err := client.DoRequest(ctx, http.MethodPost, path, headers, settings)
	if err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to install plugin: %s", string(body))
	}
	// Docker pulls the plugin while it streams the progress, and cancels the
	// pull when the client hangs up, so read the stream to the end. Failures
	// arrive inside it with status 200.
	if err := checkDockerStream(resp.Body); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	// Docker names the plugin after the remote when no alias is given.
	id := name
	if id == "" {
		id = remote
	}

	// Set before enabling, so a plugin that installed but failed to enable
	// stays in state (tainted) instead of being left behind unmanaged.
	d.SetID(id)
	if err := d.Set("name", id); err != nil {
		return err
	}

	if enable {
		// Docker requires the timeout parameter; without it the enable call
		// fails with "strconv.Atoi: parsing \"\"", so enable = true never
		// worked.
		enablePath := fmt.Sprintf("/endpoints/%d/docker/plugins/%s/enable?timeout=0", endpointID, id)
		enableResp, err := client.DoRequest(ctx, http.MethodPost, enablePath, nil, nil)
		if err != nil {
			return fmt.Errorf("plugin installed but failed to enable: %w", err)
		}
		defer enableResp.Body.Close()

		if enableResp.StatusCode >= 300 {
			body, _ := io.ReadAll(enableResp.Body)
			return fmt.Errorf("plugin installed but failed to enable: %s", string(body))
		}
	}

	return nil
}

// checkDockerStream reads a Docker progress stream to the end and returns
// the first error reported in it.
func checkDockerStream(r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("reading the progress stream: %w", err)
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
	}
}

func dockerPluginDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	plugin := d.ID()

	// force: Docker refuses to remove an enabled plugin, which made every
	// enable = true plugin impossible to destroy or replace.
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s?force=true", client.Endpoint, endpointID, plugin)
	if err := doJSON(ctx, client, http.MethodDelete, reqURL, nil, nil); err != nil && !isAPINotFound(err) {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}

	d.SetID("")
	return nil
}

func dockerPluginRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	pluginName := d.ID()
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s/json", client.Endpoint, endpointID, pluginName)

	var plugin struct {
		Enabled         bool   `json:"Enabled"`
		PluginReference string `json:"PluginReference"`
	}
	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &plugin); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read docker plugin: %w", err)
	}

	if err := d.Set("enable", plugin.Enabled); err != nil {
		return err
	}
	// Docker does not keep the remote as written nor the privileges granted at
	// install (settings), and both force a new plugin, so they keep their
	// configured values. Only an import fills them in: the remote from the
	// plugin reference and the settings from the privileges it requires.
	if dkString(d, "remote") == "" {
		remote := normalizePluginReference(plugin.PluginReference)
		fields := map[string]interface{}{"remote": remote}
		// The privileges come from the registry, which a private or
		// unreachable one may refuse; the import then just has no settings.
		if settings, err := dockerPluginPrivileges(ctx, client, endpointID, remote); err != nil {
			log.Printf("[WARN] docker_plugin %s: settings not imported: %v", pluginName, err)
		} else {
			fields["settings"] = settings
		}
		if err := d.SetFields(fields); err != nil {
			return err
		}
	}

	return nil
}

// dockerPluginPrivileges returns the privileges the plugin asks for, which
// Docker requires the install to grant exactly, in the shape of settings.
func dockerPluginPrivileges(ctx context.Context, client *APIClient, endpointID int, remote string) ([]interface{}, error) {
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/privileges?%s", client.Endpoint, endpointID, url.Values{"remote": {remote}}.Encode())
	var privileges []struct {
		Name  string   `json:"Name"`
		Value []string `json:"Value"`
	}
	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &privileges); err != nil {
		return nil, fmt.Errorf("failed to read the privileges of plugin %s: %w", remote, err)
	}
	settings := make([]interface{}, 0, len(privileges))
	for _, p := range privileges {
		values := make([]interface{}, 0, len(p.Value))
		for _, v := range p.Value {
			values = append(values, v)
		}
		settings = append(settings, map[string]interface{}{"name": p.Name, "value": values})
	}
	return settings, nil
}

// normalizePluginReference turns Docker's canonical plugin reference
// ("docker.io/rclone/docker-volume-rclone:latest") back into the short form
// users write ("rclone/docker-volume-rclone").
func normalizePluginReference(ref string) string {
	ref = strings.TrimPrefix(ref, "docker.io/")
	ref = strings.TrimPrefix(ref, "library/")
	return strings.TrimSuffix(ref, ":latest")
}

// dockerPluginUpdate only ever stores a new description of a setting, which
// Docker does not keep: there is nothing to send.
func dockerPluginUpdate(context.Context, *dkBag, *APIClient) error { return nil }

// dockerPluginImport takes "<endpoint_id>:<plugin_name>".
func dockerPluginImport(_ context.Context, d *dkBag, _ *APIClient) error {
	parts := strings.SplitN(d.ID(), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("unexpected ID format (%s), expected endpoint_id:plugin_name", d.ID())
	}
	endpointID, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid endpoint ID in ID: %w", err)
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	// The registry credentials cannot be read back; assume the default.
	if err := d.SetFields(map[string]interface{}{"name": parts[1], "registry_auth": "e30="}); err != nil {
		return err
	}
	d.SetID(parts[1])
	return nil
}

// dockerPluginSettingsPlanModifier replaces the plugin when its settings
// change, except for the description of a setting: Docker compares
// privileges by name and value only, and an import fills settings in without
// a description, so a difference in it would only replace the plugin for
// nothing (SDKv2 suppressed it). The prior description is kept where the
// configuration sets one, so such a change plans nothing.
type dockerPluginSettingsPlanModifier struct{}

func (dockerPluginSettingsPlanModifier) Description(context.Context) string {
	return "Changing a setting other than its description replaces the plugin."
}

func (m dockerPluginSettingsPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (dockerPluginSettingsPlanModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	stateV, err1 := req.StateValue.ToTerraformValue(ctx)
	planV, err2 := req.PlanValue.ToTerraformValue(ctx)
	if err1 != nil || err2 != nil {
		resp.RequiresReplace = true
		return
	}
	if !dkSemanticallyEqual(dockerPluginWithoutDescriptions(stateV), dockerPluginWithoutDescriptions(planV)) {
		resp.RequiresReplace = true
		return
	}
	var stateElems, planElems []tftypes.Value
	_ = stateV.As(&stateElems)
	_ = planV.As(&planElems)
	merged := make([]tftypes.Value, 0, len(planElems))
	for i, e := range planElems {
		attrs := dkObjectAttrs(e)
		if i < len(stateElems) {
			prior := dkObjectAttrs(stateElems[i])["description"]
			if d, ok := attrs["description"]; ok && !d.IsNull() && prior.Type() != nil && !prior.IsNull() {
				attrs["description"] = prior
			}
		}
		merged = append(merged, tftypes.NewValue(e.Type(), attrs))
	}
	list, err := dkListValueFromTF(ctx, req.PlanValue.Type(ctx), tftypes.NewValue(planV.Type(), merged))
	if err != nil {
		return
	}
	resp.PlanValue = list
}

// dockerPluginWithoutDescriptions blanks the description of every setting.
func dockerPluginWithoutDescriptions(v tftypes.Value) tftypes.Value {
	if v.IsNull() || !v.IsKnown() {
		return v
	}
	var elems []tftypes.Value
	_ = v.As(&elems)
	out := make([]tftypes.Value, 0, len(elems))
	for _, e := range elems {
		attrs := dkObjectAttrs(e)
		if attrs == nil {
			out = append(out, e)
			continue
		}
		attrs["description"] = tftypes.NewValue(tftypes.String, nil)
		out = append(out, tftypes.NewValue(e.Type(), attrs))
	}
	return tftypes.NewValue(v.Type(), out)
}
