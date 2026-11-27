package internal

// The Docker and stack resources and data sources, served by the plugin
// framework.
func init() {
	frameworkResources = append(frameworkResources,
		newStackResource,
		newDeployResource,
		newContainerExecResource,
		newDockerNodeResource,
		newDockerNetworkResource,
		newDockerImageResource,
		newDockerVolumeResource,
		newDockerPluginResource,
		newStackAssociateResource,
		newDockerSecretResource,
		newDockerConfigResource,
		newStackWebhookResource,
		newStackMigrateResource,
		newStackDeleteByNameResource,
	)
	frameworkDataSources = append(frameworkDataSources,
		newStackDataSource,
		newDockerNetworkDataSource,
		newDockerVolumeDataSource,
		newDockerConfigDataSource,
		newDockerSecretDataSource,
		newDockerImageDataSource,
		newDockerNodeDataSource,
		newStackConversionDataSource,
		newDockerSnapshotDataSource,
		newDockerSnapshotContainersDataSource,
		newDockerSnapshotContainerDataSource,
		newDockerDashboardDataSource,
		newDockerImagesDataSource,
		newDockerContainerGpusDataSource,
	)
}
