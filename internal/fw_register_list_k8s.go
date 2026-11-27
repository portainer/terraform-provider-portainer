package internal

// List resources (`terraform query`, Terraform 1.14+) of the Kubernetes
// manifest resources, the Edge stacks, jobs and configurations and the GitOps
// sources and workflows; see fw_list_resources_k8s.go.
func init() {
	frameworkListResources = append(frameworkListResources,
		newKubernetesApplicationListResource,
		newKubernetesCronJobListResource,
		newKubernetesJobListResource,
		newKubernetesServiceAccountsListResource,
		newKubernetesConfigMapsListResource,
		newKubernetesSecretListResource,
		newKubernetesServiceListResource,
		newKubernetesRoleListResource,
		newKubernetesRoleBindingListResource,
		newKubernetesClusterRoleListResource,
		newKubernetesClusterRoleBindingListResource,
		newKubernetesVolumeListResource,
		newKubernetesStorageListResource,
		newEdgeStackListResource,
		newEdgeJobListResource,
		newEdgeConfigurationsListResource,
		newGitopsSourceListResource,
		newGitopsWorkflowListResource,
	)
}
