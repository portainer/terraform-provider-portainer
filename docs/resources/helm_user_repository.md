# Resource Documentation: `portainer_helm_user_repository`

# portainer_helm_user_repository
The `portainer_helm_user_repository` resource manages per-user Helm chart repositories in Portainer.

## Example Usage

```hcl
resource "portainer_helm_user_repository" "bitnami" {
  user_id = 1
  url     = "https://charts.bitnami.com/bitnami"
}
```

## Lifecycle & Behavior
This resource creates a Helm repository association for a specific user. Deleting the resource removes the repository from the user's list. The URL and user_id are immutable; changing either forces recreation.

## Arguments Reference

| Name      | Type   | Required | Description                                                         |
|-----------|--------|----------|---------------------------------------------------------------------|
| `user_id` | number | Yes      | User identifier.                                                    |
| `url`     | string | Yes      | Helm repository URL (e.g. `https://charts.bitnami.com/bitnami`).   |

## Attributes Reference

| Name | Type   | Description                               |
|------|--------|-------------------------------------------|
| `id` | string | The ID of the Helm user repository entry. |

## Import

Helm user repositories can be imported using the repository ID:

```bash
terraform import portainer_helm_user_repository.bitnami 5
```

Note: After import, you must set `user_id` in config for subsequent reads to work.
