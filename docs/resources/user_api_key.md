# Resource Documentation: `portainer_user_api_key`

# portainer_user_api_key
The `portainer_user_api_key` resource manages a Portainer API key for a user, with a full lifecycle: it can be created **and revoked**.

This closes a gap in `portainer_user`, whose `generate_api_key` argument can issue a key but never remove one — rotating a key there meant recreating the whole user.

## Example Usage

```hcl
resource "portainer_user_api_key" "terraform" {
  user_id     = portainer_user.ci.id
  description = "terraform"
  password    = var.ci_user_password
}

output "ci_api_key" {
  value     = portainer_user_api_key.terraform.raw_api_key
  sensitive = true
}
```

Rotating a key is a `terraform apply -replace`, or a change to `description`: every argument forces a new key, because Portainer has no update for one.

## Arguments Reference

| Name          | Type   | Required | Description                                                                                                                    |
|---------------|--------|----------|--------------------------------------------------------------------------------------------------------------------------------|
| `user_id`     | number | ✅ yes   | Identifier of the user the key belongs to. Changing it replaces the resource.                                                   |
| `description` | string | ✅ yes   | Description shown in Portainer's user settings. Portainer requires it to be unique per user. Changing it replaces the resource. |
| `password`    | string | ✅ yes   | Password of that user, which Portainer requires to authorize the request. Stored in state as a sensitive value.                 |

## Attributes Reference

| Name           | Description                                                                                                                              |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `id`           | Identifier of the API key.                                                                                                                  |
| `raw_api_key`  | The generated key. **Portainer returns it exactly once**, at creation, so it only exists in the state written by that apply — treat the state as a secret. |
| `prefix`       | Non-secret prefix Portainer displays to identify the key.                                                                                   |
| `date_created` | Unix timestamp at which the key was created.                                                                                                |
| `last_used`    | Unix timestamp of the key's last use, zero when it has never been used.                                                                     |

A key revoked in the Portainer UI disappears from state on the next refresh, and the next apply issues a new one.
