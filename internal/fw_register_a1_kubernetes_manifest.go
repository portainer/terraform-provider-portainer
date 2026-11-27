package internal

func init() {
	frameworkResources = append(frameworkResources,
		newKubernetesApplicationResource,
		newKubernetesCronJobResource,
		newKubernetesJobResource,
		newKubernetesServiceAccountsResource,
		newKubernetesConfigMapsResource,
		newKubernetesSecretResource,
		newKubernetesServiceResource,
		newKubernetesRoleResource,
		newKubernetesRoleBindingResource,
		newKubernetesClusterRoleResource,
		newKubernetesClusterRoleBindingResource,
		newKubernetesVolumeResource,
		newKubernetesStorageResource,
	)
}
