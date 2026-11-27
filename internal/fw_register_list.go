package internal

// List resources (`terraform query`, Terraform 1.14+) of the long-lived
// resources with the { id } identity; see fw_list_resources.go.
func init() {
	frameworkListResources = append(frameworkListResources,
		newTagListResource,
		newTeamListResource,
		newUserListResource,
		newEndpointGroupListResource,
		newEdgeGroupListResource,
		newEnvironmentListResource,
		newRegistryListResource,
		newCustomTemplateListResource,
		newWebhookListResource,
	)
}
