package internal

// Actions (Terraform >= 1.14; OpenTofu has none): the one-shot operations of
// the action resources, invoked from a lifecycle action_trigger or with
// `terraform apply -invoke`. The resources stay for OpenTofu and older
// Terraform.
func init() {
	frameworkActions = append(frameworkActions,
		newEndpointSnapshotAction,
		newBackupAction,
		newBackupS3Action,
		newBackupLocalRunAction,
		newBackupAzureExecuteAction,
		newWebhookExecuteAction,
		newEdgeStackWebhookAction,
		newContainerExecAction,
		newDeployAction,
		newCheckAction,
		newEndpointServiceUpdateAction,
		newEndpointAssociationAction,
		newKubernetesNodeDrainAction,
		newKubernetesDeploymentScaleAction,
		newKubernetesDeploymentRollbackAction,
		newKubernetesClusterUpgradeAction,
		newKubernetesDeleteObjectAction,
		newStackMigrateAction,
		newStackAssociateAction,
		newStackDeleteByNameAction,
		newAddonRepairAction,
		newAddonCatalogRefreshAction,
		newGitopsWorkflowReconcileAction,
		newOmniNodeRebootAction,
		newHelmRollbackAction,
		newUserMembershipsSyncAction,
	)
}
