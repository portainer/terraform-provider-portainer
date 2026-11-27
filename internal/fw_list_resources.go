package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// The list resources of `terraform query` (Terraform 1.14+). Each lists the
// objects of the resource type of the same name with one API call and
// filters them here; the full resource of a result comes from the resource's
// Read (fwListReadResource).

func fwID(id int64) string { return strconv.FormatInt(id, 10) }

var newTagListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_tag",
	description: "Lists Portainer tags.",
	newResource: newTagResource,
	filters:     fwListNameFilter("tags"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		tags, err := listTags(ctx, client)
		if err != nil {
			diags.AddError("Failed to list tags", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, t := range tags {
			if name == "" || t.Name == name {
				out = append(out, fwIDListItem{id: fwID(t.ID), name: t.Name})
			}
		}
		return out
	},
})

var newTeamListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_team",
	description: "Lists Portainer teams.",
	newResource: newTeamResource,
	filters:     fwListNameFilter("teams"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		teams, err := listTeams(ctx, client)
		if err != nil {
			diags.AddError("Failed to list teams", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, t := range teams {
			if name == "" || t.Name == name {
				out = append(out, fwIDListItem{id: fwID(t.ID), name: t.Name})
			}
		}
		return out
	},
})

var newUserListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_user",
	description: "Lists Portainer users. Passwords and API keys are never part of the results.",
	newResource: newUserResource,
	filters: map[string]listschema.Attribute{
		"username": listschema.StringAttribute{Optional: true, Description: "Only list the user with exactly this username."},
		"role":     listschema.Int64Attribute{Optional: true, Description: "Only list the users with this role: 1 = administrator, 2 = standard user."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		username := fwListFilter(ctx, config, "username", diags)
		role := fwListFilterInt64(ctx, config, "role", diags)
		users, err := listUsers(ctx, client)
		if err != nil {
			diags.AddError("Failed to list users", fmt.Sprintf("failed to list users: %s", err))
			return nil
		}
		var out []fwIDListItem
		for _, u := range users {
			if (username == "" || u.Username == username) && (role == 0 || u.Role == role) {
				out = append(out, fwIDListItem{id: fwID(u.ID), name: u.Username})
			}
		}
		return out
	},
})

var newEndpointGroupListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_endpoint_group",
	description: "Lists Portainer environment (endpoint) groups, the built-in Unassigned group included.",
	newResource: newEndpointGroupResource,
	filters:     fwListNameFilter("environment groups"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		groups, err := listEndpointGroups(ctx, client)
		if err != nil {
			diags.AddError("Failed to list endpoint groups", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, g := range groups {
			if name == "" || g.Name == name {
				out = append(out, fwIDListItem{id: fwID(g.ID), name: g.Name})
			}
		}
		return out
	},
})

var newEdgeGroupListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_edge_group",
	description: "Lists Portainer Edge groups.",
	newResource: newEdgeGroupResource,
	filters:     fwListNameFilter("Edge groups"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		var groups []struct {
			ID   int64  `json:"Id"`
			Name string `json:"Name"`
		}
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/edge_groups", nil, &groups); err != nil {
			diags.AddError("Failed to list edge groups", fmt.Sprintf("failed to list edge groups: %s", err))
			return nil
		}
		var out []fwIDListItem
		for _, g := range groups {
			if name == "" || g.Name == name {
				out = append(out, fwIDListItem{id: fwID(g.ID), name: g.Name})
			}
		}
		return out
	},
})

var newEnvironmentListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_environment",
	description: "Lists Portainer environments (endpoints). TLS keys and other write-only settings are never part of the results.",
	newResource: newEnvironmentResource,
	filters: map[string]listschema.Attribute{
		"name":     listschema.StringAttribute{Optional: true, Description: "Only list the environment with exactly this name."},
		"group_id": listschema.Int64Attribute{Optional: true, Description: "Only list the environments of this environment group."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		groupID := fwListFilterInt64(ctx, config, "group_id", diags)
		envs, err := listEnvironments(ctx, client)
		if err != nil {
			diags.AddError("Failed to list environments", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, e := range envs {
			if (name == "" || e.Name == name) && (groupID == 0 || e.GroupID == groupID) {
				out = append(out, fwIDListItem{id: fwID(e.ID), name: e.Name})
			}
		}
		return out
	},
})

var newRegistryListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_registry",
	description: "Lists Portainer registries. Passwords are never part of the results.",
	newResource: newRegistryResource,
	filters: map[string]listschema.Attribute{
		"name": listschema.StringAttribute{Optional: true, Description: "Only list the registry with exactly this name."},
		"type": listschema.Int64Attribute{Optional: true, Description: "Only list the registries of this type (1 = Quay, 2 = Azure, 3 = custom, 4 = GitLab, 5 = ProGet, 6 = Docker Hub, 7 = ECR, 8 = GitHub)."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		typ := fwListFilterInt64(ctx, config, "type", diags)
		registries, err := listRegistries(ctx, client)
		if err != nil {
			diags.AddError("Failed to list registries", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, r := range registries {
			if (name == "" || r.Name == name) && (typ == 0 || r.Type == typ) {
				out = append(out, fwIDListItem{id: fwID(r.ID), name: r.Name})
			}
		}
		return out
	},
})

var newCustomTemplateListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_custom_template",
	description: "Lists Portainer custom templates. The template content and repository credentials are not part of the results.",
	newResource: newCustomTemplateResource,
	filters: map[string]listschema.Attribute{
		"title": listschema.StringAttribute{Optional: true, Description: "Only list the custom templates with exactly this title."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		title := fwListFilter(ctx, config, "title", diags)
		templates, err := listCustomTemplates(ctx, client)
		if err != nil {
			diags.AddError("Failed to list custom templates", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, t := range templates {
			if title == "" || t.Title == title {
				out = append(out, fwIDListItem{id: fwID(t.ID), name: t.Title})
			}
		}
		return out
	},
})

var newWebhookListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_webhook",
	description: "Lists Portainer service and container webhooks. Business Edition only lists the webhooks of one resource: set both endpoint_id and resource_id there.",
	newResource: newWebhookResource,
	filters: map[string]listschema.Attribute{
		"endpoint_id": listschema.Int64Attribute{Optional: true, Description: "Only list the webhooks of this environment (endpoint)."},
		"resource_id": listschema.StringAttribute{Optional: true, Description: "Only list the webhooks of this resource (service or container ID)."},
	},
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		endpointID := fwListFilterInt64(ctx, config, "endpoint_id", diags)
		resourceID := fwListFilter(ctx, config, "resource_id", diags)
		// Portainer only filters by both values together; Business Edition
		// cannot list without that filter at all.
		var webhooks []webhookRecord
		var err error
		if endpointID != 0 && resourceID != "" {
			webhooks, err = listWebhooks(ctx, client, int(endpointID), resourceID)
		} else {
			webhooks, err = listWebhooks(ctx, client, 0, "")
		}
		if err != nil {
			diags.AddError("Failed to list webhooks", err.Error()+"\n\nOn Business Edition set both endpoint_id and resource_id.")
			return nil
		}
		var out []fwIDListItem
		for _, w := range webhooks {
			if (endpointID != 0 && int64(w.EndpointID) != endpointID) || (resourceID != "" && w.ResourceID != resourceID) {
				continue
			}
			// Read looks the webhook up under its resource, which Business
			// Edition needs.
			out = append(out, fwIDListItem{
				id:   strconv.Itoa(w.ID),
				name: fmt.Sprintf("webhook %d (%s)", w.ID, w.ResourceID),
				seed: map[string]any{"endpoint_id": int64(w.EndpointID), "resource_id": w.ResourceID},
			})
		}
		return out
	},
})
