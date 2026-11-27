# Ephemeral `portainer_kubernetes_scoped_kubeconfig`

Issues a namespace-scoped, read-only kubeconfig for one run, configures the `kubernetes` provider
with it, and revokes it when the run ends. Nothing secret is written to the plan or the state.

Requires Terraform >= 1.10 or OpenTofu >= 1.11 and Portainer Business Edition 2.45.2 or later. See
[docs/ephemeral-resources/kubernetes_scoped_kubeconfig.md](../../docs/ephemeral-resources/kubernetes_scoped_kubeconfig.md).
