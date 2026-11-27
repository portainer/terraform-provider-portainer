package internal

// Batch A2 of the plugin-framework migration: the Kubernetes resources that
// are not defined by a manifest.
func init() {
	frameworkResources = append(frameworkResources,
		newKubernetesPodSecurityRuleResource,
		newKubernetesClusterUpgradeResource,
		newKubernetesNamespaceResource,
		newKubernetesHelmResource,
		newKubernetesIngressControllersResource,
		newKubernetesEnvironmentConfigResource,
		newKubernetesNamespaceIngressControllersResource,
		newKubernetesIngressResource,
		newKubernetesGatewayResource,
		newKubernetesGatewayHTTPRouteResource,
		newKubernetesNamespaceSystemResource,
		newKubernetesDeleteObjectResource,
		newKubernetesNodeDrainResource,
		newKubernetesDeploymentScaleResource,
		newKubernetesDeploymentRollbackResource,
		newKubernetesNamespaceAccessResource,
		newKubernetesScopedKubeconfigResource,
		newKubernetesPersistentVolumeResource,
	)
}
