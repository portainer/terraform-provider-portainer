# Data Source Documentation: `portainer_kubernetes_service_account`

# portainer_kubernetes_service_account

Inspects one Kubernetes service account.

## Example Usage

```hcl
data "portainer_kubernetes_service_account" "deployer" {
  endpoint_id = portainer_environment.prod.id
  namespace   = "apps"
  name        = "deployer"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Kubernetes environment to query. |
| `name` | string | ✅ yes | Name of the service account to inspect. |
| `namespace` | string | ✅ yes | Namespace of the service account. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `annotations` | map(string) | Annotations on the service account. |
| `automount_service_account_token` | bool | Whether pods using the account get its token mounted automatically. |
| `creation_date` | string | When the service account was created. |
| `image_pull_secrets` | list(string) | Image pull secrets attached to the account. |
| `is_system` | bool | Whether this is one of Kubernetes' own system service accounts. |
| `labels` | map(string) | Labels on the service account. |
| `uid` | string | Kubernetes UID of the service account. |

`image_pull_secrets` lists the secrets by name; it never carries their contents.
