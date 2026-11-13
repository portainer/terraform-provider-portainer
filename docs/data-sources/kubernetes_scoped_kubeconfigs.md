# Data Source Documentation: `portainer_kubernetes_scoped_kubeconfigs`

# portainer_kubernetes_scoped_kubeconfigs

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

Lists issued scoped kubeconfigs (see `portainer_kubernetes_scoped_kubeconfig`), newest first, for one environment or across all of them. A user sees the ones they issued; an administrator sees every one. Expired kubeconfigs are listed with `active = false` until the next issue on their environment sweeps them; a revoked one is listed only when its Kubernetes objects could not be removed.

Portainer never returns the credential after issuing it, so nothing in this data source is secret.

## Example Usage

```hcl
data "portainer_kubernetes_scoped_kubeconfigs" "all" {}

data "portainer_kubernetes_scoped_kubeconfigs" "prod" {
  environment_id = 4
}

output "active_scoped_kubeconfigs" {
  value = [for k in data.portainer_kubernetes_scoped_kubeconfigs.all.scoped_kubeconfigs : "${k.username}: ${k.description}" if k.active]
}
```

## Arguments Reference

| Name             | Type   | Required | Description |
|------------------|--------|----------|-------------|
| `environment_id` | number | No       | Environment to list the scoped kubeconfigs of; unset lists every environment. |

## Attributes Reference

| Name                 | Type | Description |
|----------------------|------|-------------|
| `scoped_kubeconfigs` | list | Scoped kubeconfigs, each with `id`, `environment_id`, `user_id`, `username`, `description`, `namespaces`, `include_secrets`, `service_account`, `active`, `created_at` and `expires_at` (Unix timestamps). |
