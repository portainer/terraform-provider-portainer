# Resource Documentation: `portainer_user_api_key`

# portainer_user_api_key

> **The provider has to be authenticated with `api_user` and `api_password`, not `api_key`.**
> Portainer accepts `POST /users/{id}/tokens` only from a session and only for the calling
> user's own account. With `api_key` it answers `401 Auth not supported`, and an administrator
> cannot mint a key on someone else's behalf.
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

## Authentication

Portainer only lets a user create a key for themselves, and a provider authenticates when
Terraform configures it - before any resource exists. A user created by the same
configuration can therefore never be signed in as, not even through a second aliased
provider: the alias would try to log in during the plan, while the user does not exist yet.

So the account the key is for has to exist beforehand, and the provider has to be signed in
as that account:

```hcl
provider "portainer" {
  endpoint     = var.portainer_url
  api_user     = var.username
  api_password = var.password
}

data "portainer_user" "self" {
  username = var.username
}

resource "portainer_user_api_key" "key" {
  user_id     = tonumber(data.portainer_user.self.id)
  description = "ci"
  password    = var.password
}
```

To issue a key for a user Terraform creates, split it across two applies: create the user in
one configuration, then run a second one signed in as them.
