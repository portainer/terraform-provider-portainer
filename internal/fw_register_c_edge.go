package internal

// Framework registrations of the edge, D2K and KubeSolo resources and data
// sources (moved from the SDKv2 half).
func init() {
	frameworkResources = append(frameworkResources,
		newEdgeGroupResource,
		newEdgeJobResource,
		newEdgeUpdateSchedulesResource,
		newEdgeStackWebhookResource,
		newEdgeJobTaskLogsResource,
		newD2KEnvironmentResource,
		newKubeSoloOfflineBundleResource,
		newEdgeConfigurationsResource,
		newEdgeStackResource,
	)
	frameworkDataSources = append(frameworkDataSources,
		newKubeSoloVersionsDataSource,
		newKubeSoloOfflineBundlesDataSource,
		newKubeSoloConfigDataSource,
		newEdgeGroupDataSource,
		newEdgeStackDataSource,
		newEdgeJobDataSource,
		newEdgeConfigurationDataSource,
		newEdgeMTLSCACertificateDataSource,
		newEdgeMTLSCertificateDataSource,
		newEdgeWaitingRoomDataSource,
		newEdgeUpdateScheduleInfoDataSource,
		newEdgeUpdatePreviousVersionsDataSource,
		newEdgeUpdateSchedulesActiveDataSource,
		newEdgeConfigurationFilesDataSource,
		newEdgeStackStaggerStatusDataSource,
		newEdgeStackFileDataSource,
		newEdgeJobFileDataSource,
		newEdgeJobTasksDataSource,
		newEdgeJobTaskLogsDataSource,
	)
}
