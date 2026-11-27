package internal

// List resources (`terraform query`, Terraform 1.14+) of the Docker
// resources and portainer_stack; see fw_list_resources_docker.go.
func init() {
	frameworkListResources = append(frameworkListResources,
		newDockerNetworkListResource,
		newDockerVolumeListResource,
		newDockerSecretListResource,
		newDockerConfigListResource,
		newDockerPluginListResource,
		newDockerNodeListResource,
		newStackListResource,
	)
}
