package internal

// Batch E: environments, endpoint groups and settings, registries, webhooks
// and resource controls.
func init() {
	frameworkResources = append(frameworkResources,
		newEnvironmentResource,
		newEndpointGroupResource,
		newEndpointGroupAccessResource,
		newEndpointGroupMembershipResource,
		newEndpointRelationsResource,
		newEndpointAssociationResource,
		newEndpointSettingsResource,
		newEndpointTrustResource,
		newEndpointSnapshotResource,
		newEndpointServiceUpdateResource,
		newEdgeGenerateKeyResource,
		newRegistryResource,
		newRegistryAccessResource,
		newRegistryConfigureResource,
		newWebhookResource,
		newWebhookExecuteResource,
		newResourceControlResource,
	)
	frameworkDataSources = append(frameworkDataSources,
		newEnvironmentDataSource,
		newEndpointGroupDataSource,
		newEndpointGroupAccessDataSource,
		newEndpointMTLSCertificateDataSource,
		newEndpointMTLSCertificateErrorDataSource,
		newEndpointsSummaryDataSource,
		newEndpointRegistriesDataSource,
		newEnvironmentLogsDataSource,
		newEnvironmentMetricsDataSource,
		newRegistryDataSource,
		newRegistryAccessDataSource,
		newRegistryConnectionDataSource,
		newTagDataSource,
		newWebhookDataSource,
	)
}
