# Data Source Documentation: `portainer_kubernetes_config`

# portainer_kubernetes_config
Generates a kubeconfig for the Kubernetes environments the calling user can reach, the same file Portainer's UI offers for download.

## Example Usage

```hcl
data "portainer_kubernetes_config" "prod" {
  environment_ids = [1, 2]
}

resource "local_sensitive_file" "kubeconfig" {
  filename = "${path.module}/kubeconfig.yaml"
  content  = data.portainer_kubernetes_config.prod.kubeconfig
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_ids` | list | ❌ no | Environments to include. Leave unset to include every Kubernetes environment the token can reach. |
| `exclude_environment_ids` | list | ❌ no | Environments to leave out. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `kubeconfig` | string | The generated kubeconfig, as YAML. |

**The kubeconfig carries a bearer token for the calling user**, so it is a credential in its own right. It is marked sensitive, but it still lands in Terraform state — treat that state accordingly, and prefer a short `kubeconfig_expiry` in Portainer's settings.
