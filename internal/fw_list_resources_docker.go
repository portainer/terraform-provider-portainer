package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// List resources (`terraform query`, Terraform 1.14+) of the bag-backed
// Docker resources and portainer_stack, whose identities are composite (see
// fw_d_docker_identity.go). A result carries the identity; the full resource,
// when asked for, is what an import by that identity followed by Read gives.

// dkListItem is one object a list resource found.
type dkListItem struct {
	name     string
	identity map[string]any
}

type dkListResource struct {
	typeSuffix  string
	description string
	newResource func() resource.Resource
	identity    *dkIdentityDef
	filters     map[string]listschema.Attribute
	items       func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []dkListItem

	client *APIClient
}

var (
	_ list.ListResource              = (*dkListResource)(nil)
	_ list.ListResourceWithConfigure = (*dkListResource)(nil)
)

func dkNewList(def dkListResource) func() list.ListResource {
	return func() list.ListResource {
		r := def
		return &r
	}
}

func (r *dkListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *dkListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{Description: r.description, Attributes: r.filters}
}

func (r *dkListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *dkListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var diags diag.Diagnostics
	if r.client == nil {
		diags.AddError("Provider not configured", "The list resource needs a configured portainer provider.")
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}
	items := r.items(ctx, r.client, req.Config, &diags)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	var res resource.Resource
	if req.IncludeResource {
		res = r.newResource()
		if c, ok := res.(resource.ResourceWithConfigure); ok {
			var cresp resource.ConfigureResponse
			c.Configure(ctx, resource.ConfigureRequest{ProviderData: r.client}, &cresp)
			if cresp.Diagnostics.HasError() {
				stream.Results = list.ListResultsStreamDiagnostics(cresp.Diagnostics)
				return
			}
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, item := range items {
			result := req.NewListResult(ctx)
			result.DisplayName = item.name
			r.identity.set(ctx, item.identity, result.Identity, &result.Diagnostics)
			if res != nil && !result.Diagnostics.HasError() && !dkListReadResource(ctx, res, &result) {
				continue // gone between the listing and the read
			}
			if !push(result) {
				return
			}
		}
	}
}

// dkListReadResource fills result.Resource with what importing the result's
// identity and reading it stores. It reports false when Read found the
// object gone.
func dkListReadResource(ctx context.Context, res resource.Resource, result *list.ListResult) bool {
	importer, ok := res.(resource.ResourceWithImportState)
	if !ok {
		result.Diagnostics.AddError("Resource cannot be imported", "The list resource needs an importable resource.")
		return true
	}
	identity := &tfsdk.ResourceIdentity{Schema: result.Identity.Schema, Raw: result.Identity.Raw.Copy()}
	empty := tftypes.NewValue(result.Resource.Schema.Type().TerraformType(ctx), nil)
	importResp := resource.ImportStateResponse{
		State:    tfsdk.State{Schema: result.Resource.Schema, Raw: empty},
		Identity: identity,
	}
	importer.ImportState(ctx, resource.ImportStateRequest{Identity: identity}, &importResp)
	result.Diagnostics.Append(importResp.Diagnostics...)
	if result.Diagnostics.HasError() {
		return true
	}

	readReq := resource.ReadRequest{State: importResp.State, Identity: importResp.Identity}
	readResp := resource.ReadResponse{State: importResp.State, Identity: importResp.Identity}
	res.Read(ctx, readReq, &readResp)
	result.Diagnostics.Append(readResp.Diagnostics...)
	if result.Diagnostics.HasError() {
		return true
	}
	if readResp.State.Raw.IsNull() {
		return false
	}
	result.Resource.Raw = readResp.State.Raw
	return true
}

// dkListEndpointFilters are the filters of a Docker list resource: the
// environment (required, Docker objects live in one) and an exact name.
func dkListEndpointFilters(what string) map[string]listschema.Attribute {
	return map[string]listschema.Attribute{
		"endpoint_id": listschema.Int64Attribute{Required: true, Description: fmt.Sprintf("ID of the Portainer environment (endpoint) whose %s are listed.", what)},
		"name":        listschema.StringAttribute{Optional: true, Description: fmt.Sprintf("Only list the %s with exactly this name.", what)},
	}
}

// dkListDocker lists the Docker objects of what under
// /endpoints/<endpoint_id>/docker/<apiPath>: pairs turns the response into
// (key, name) pairs, key being the second identity part (keyAttr).
func dkListDocker[T any](what, apiPath, keyAttr string, pairs func(T) [][2]string) func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []dkListItem {
	return func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []dkListItem {
		endpointID := fwListFilterInt64(ctx, config, "endpoint_id", diags)
		name := fwListFilter(ctx, config, "name", diags)
		if diags.HasError() {
			return nil
		}
		var resp T
		if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/%s", client.Endpoint, endpointID, apiPath), nil, &resp); err != nil {
			diags.AddError("Failed to list Docker "+what, fmt.Sprintf("failed to list the Docker %s of environment %d: %s", what, endpointID, err))
			return nil
		}
		var out []dkListItem
		for _, p := range pairs(resp) {
			if p[0] == "" || (name != "" && p[1] != name) {
				continue
			}
			out = append(out, dkListItem{name: p[1], identity: map[string]any{"endpoint_id": endpointID, keyAttr: p[0]}})
		}
		return out
	}
}

// The entries of the Docker listings, as far as a list resource needs them.
type dockerNetworkListEntry struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

type dockerVolumeListResponse struct {
	Volumes []struct {
		Name string `json:"Name"`
	} `json:"Volumes"`
}

type dockerPluginListEntry struct {
	Name string `json:"Name"`
}

type dockerNodeListEntry struct {
	ID          string `json:"ID"`
	Description struct {
		Hostname string `json:"Hostname"`
	} `json:"Description"`
}

// dockerSwarmObject is a secret or config of a Swarm listing.
type dockerSwarmObject struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

func dockerSwarmObjectPairs(objs []dockerSwarmObject) [][2]string {
	out := make([][2]string, 0, len(objs))
	for _, o := range objs {
		out = append(out, [2]string{o.ID, o.Spec.Name})
	}
	return out
}

var newDockerNetworkListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_network",
	description: "Lists the Docker networks of a Portainer environment, the predefined ones (bridge, host, none, ingress) included.",
	newResource: newDockerNetworkResource,
	identity:    dockerNetworkIdentity,
	filters:     dkListEndpointFilters("networks"),
	items: dkListDocker("networks", "networks", "id", func(nets []dockerNetworkListEntry) [][2]string {
		out := make([][2]string, 0, len(nets))
		for _, n := range nets {
			out = append(out, [2]string{n.ID, n.Name})
		}
		return out
	}),
})

var newDockerVolumeListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_volume",
	description: "Lists the Docker volumes of a Portainer environment.",
	newResource: newDockerVolumeResource,
	identity:    dockerVolumeIdentity,
	filters:     dkListEndpointFilters("volumes"),
	items: dkListDocker("volumes", "volumes", "name", func(resp dockerVolumeListResponse) [][2]string {
		out := make([][2]string, 0, len(resp.Volumes))
		for _, v := range resp.Volumes {
			out = append(out, [2]string{v.Name, v.Name})
		}
		return out
	}),
})

var newDockerSecretListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_secret",
	description: "Lists the Docker Swarm secrets of a Portainer environment. Docker never returns secret data, so it is not part of the results.",
	newResource: newDockerSecretResource,
	identity:    dockerSecretIdentity,
	filters:     dkListEndpointFilters("secrets"),
	items:       dkListDocker("secrets", "secrets", "id", dockerSwarmObjectPairs),
})

var newDockerConfigListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_config",
	description: "Lists the Docker Swarm configs of a Portainer environment. The config data is not read back, so it is not part of the results.",
	newResource: newDockerConfigResource,
	identity:    dockerConfigIdentity,
	filters:     dkListEndpointFilters("configs"),
	items:       dkListDocker("configs", "configs", "id", dockerSwarmObjectPairs),
})

var newDockerPluginListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_plugin",
	description: "Lists the Docker plugins installed on a Portainer environment. Names are listed without Docker's :latest tag, as the resource stores them.",
	newResource: newDockerPluginResource,
	identity:    dockerPluginIdentity,
	filters:     dkListEndpointFilters("plugins"),
	items: dkListDocker("plugins", "plugins", "name", func(plugins []dockerPluginListEntry) [][2]string {
		out := make([][2]string, 0, len(plugins))
		for _, p := range plugins {
			name := strings.TrimSuffix(p.Name, ":latest")
			out = append(out, [2]string{name, name})
		}
		return out
	}),
})

var newDockerNodeListResource = dkNewList(dkListResource{
	typeSuffix:  "_docker_node",
	description: "Lists the nodes of the Docker Swarm of a Portainer environment. The display name is the node's hostname.",
	newResource: newDockerNodeResource,
	identity:    dockerNodeIdentity,
	filters: map[string]listschema.Attribute{
		"endpoint_id": listschema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (endpoint, a Docker Swarm manager) whose nodes are listed."},
		"name":        listschema.StringAttribute{Optional: true, Description: "Only list the node with exactly this hostname."},
	},
	items: dkListDocker("nodes", "nodes", "node_id", func(nodes []dockerNodeListEntry) [][2]string {
		out := make([][2]string, 0, len(nodes))
		for _, n := range nodes {
			name := n.Description.Hostname
			if name == "" {
				name = n.ID
			}
			out = append(out, [2]string{n.ID, name})
		}
		return out
	}),
})

// stackDeploymentTypes maps Portainer's stack type to deployment_type.
var stackDeploymentTypes = map[int]string{1: "swarm", 2: "standalone", 3: "kubernetes"}

var newStackListResource = dkNewList(dkListResource{
	typeSuffix: "_stack",
	description: "Lists Portainer stacks (Docker standalone, Docker Swarm and Kubernetes; Edge stacks are portainer_edge_stack). " +
		"Portainer does not record whether a stack came from a string, a file or a URL: with include_resource, method is " +
		"\"repository\" for a stack deployed from Git and \"string\" for any other, with stack_file_content read from Portainer.",
	newResource: newStackResource,
	identity:    stackIdentity,
	filters: map[string]listschema.Attribute{
		"endpoint_id":     listschema.Int64Attribute{Optional: true, Description: "Only list the stacks of this environment (endpoint)."},
		"name":            listschema.StringAttribute{Optional: true, Description: "Only list the stacks with exactly this name."},
		"deployment_type": listschema.StringAttribute{Optional: true, Description: "Only list the stacks of this deployment mode: standalone, swarm or kubernetes."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []dkListItem {
		endpointID := fwListFilterInt64(ctx, config, "endpoint_id", diags)
		name := fwListFilter(ctx, config, "name", diags)
		typ := fwListFilter(ctx, config, "deployment_type", diags)
		if diags.HasError() {
			return nil
		}
		var stacks []struct {
			ID         int64  `json:"Id"`
			Name       string `json:"Name"`
			Type       int    `json:"Type"`
			EndpointID int64  `json:"EndpointId"`
		}
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/stacks", nil, &stacks); err != nil {
			diags.AddError("Failed to list stacks", fmt.Sprintf("failed to list stacks: %s", err))
			return nil
		}
		var out []dkListItem
		for _, s := range stacks {
			deploymentType, known := stackDeploymentTypes[s.Type]
			if !known || (endpointID != 0 && s.EndpointID != endpointID) || (name != "" && s.Name != name) || (typ != "" && deploymentType != typ) {
				continue
			}
			out = append(out, dkListItem{
				name:     s.Name,
				identity: map[string]any{"endpoint_id": s.EndpointID, "id": fwID(s.ID), "deployment_type": deploymentType},
			})
		}
		return out
	},
})
