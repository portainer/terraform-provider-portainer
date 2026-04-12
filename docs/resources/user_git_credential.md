# Resource Documentation: `portainer_user_git_credential`

# portainer_user_git_credential
The `portainer_user_git_credential` resource allows you to manage user-level git credentials in Portainer. User git credentials are private to a specific user.

> Requires Portainer Business Edition 2.39+

## Example Usage

### Basic authentication
```hcl
resource "portainer_user_git_credential" "example" {
  user_id            = 1
  name               = "my-git-credential"
  username           = "git-user"
  password           = "git-password"
  authorization_type = 0
}
```

### Token authentication
```hcl
resource "portainer_user_git_credential" "token_example" {
  user_id            = 1
  name               = "my-git-token"
  username           = "git-user"
  password           = "ghp_xxxxxxxxxxxxxxxxxxxx"
  authorization_type = 1
}
```

## Lifecycle & Behavior
User git credentials are created via the `/users/{id}/gitcredentials` API.

Only the owning user can create, read, update, or delete their own git credentials.

The `password` field is sensitive and will not be displayed in plan output.

The resource ID in Terraform state uses a composite format: `<user_id>:<credential_id>`.

- To destroy a user git credential:
```hcl
terraform destroy
```

- To update a user git credential, update the relevant fields and re-apply:
```hcl
terraform apply
```

## Arguments Reference
| **Name**             | **Type** | **Required** | **Description**                                           |
|----------------------|----------|--------------|-----------------------------------------------------------|
| `user_id`            | int      | yes          | ID of the user who owns this credential (ForceNew)        |
| `name`               | string   | yes          | Name of the git credential                                |
| `username`           | string   | yes          | Username for git authentication                           |
| `password`           | string   | yes          | Password or token for git authentication (sensitive)      |
| `authorization_type` | int      | no           | Authorization type: `0` = Basic (default), `1` = Token    |

## Attributes Reference
| Name            | Description                          |
|-----------------|--------------------------------------|
| `id`            | Composite ID (`user_id:credential_id`) |
| `credential_id` | ID of the git credential             |

## Import

User git credentials can be imported using the composite format `<user_id>:<credential_id>`:

```shell
terraform import portainer_user_git_credential.example 1:123
```
