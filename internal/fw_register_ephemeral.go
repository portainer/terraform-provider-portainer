package internal

// Ephemeral resources: credentials handed to other providers or modules for
// one run, never written to the plan or the state (Terraform >= 1.10,
// OpenTofu >= 1.11).
func init() {
	frameworkEphemeralResources = append(frameworkEphemeralResources,
		newUserAPIKeyEphemeral,
		newScopedKubeconfigEphemeral,
		newKubernetesConfigEphemeral,
		newAuthEphemeral,
		newEdgeGenerateKeyEphemeral,
		newSSHKeygenEphemeral,
	)
}
