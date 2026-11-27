package internal

// Portainer's built-in OIDC provider (Business Edition 3.1+): its clients,
// their secrets and sessions, the signing key and the launcher.
func init() {
	frameworkResources = append(frameworkResources,
		newOIDCClientResource,
		newOIDCClientSecretResource,
		newOIDCKeysRotateResource,
		newOIDCClientSessionsEndResource,
		newOIDCUserSessionsEndResource,
	)
	frameworkDataSources = append(frameworkDataSources,
		newOIDCClientsDataSource,
		newOIDCClientSessionsDataSource,
		newOIDCLauncherDataSource,
	)
	frameworkActions = append(frameworkActions,
		newOIDCKeysRotateAction,
		newOIDCClientSessionsEndAction,
		newOIDCUserSessionsEndAction,
	)
	frameworkListResources = append(frameworkListResources,
		newOIDCClientListResource,
	)
}
