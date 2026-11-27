# Ephemeral `portainer_kubernetes_config`

Generates a kubeconfig through Portainer and configures the `kubernetes` provider with it, without
writing the kubeconfig to the plan or the state (unlike the `portainer_kubernetes_config` data source).

Requires Terraform >= 1.10 or OpenTofu >= 1.11. See
[docs/ephemeral-resources/kubernetes_config.md](../../docs/ephemeral-resources/kubernetes_config.md).
