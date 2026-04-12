# Resource Documentation: `portainer_shared_git_credential`

# portainer_shared_git_credential
The `portainer_shared_git_credential` resource allows you to manage shared git credentials in Portainer. Shared git credentials are available to all users and are managed by administrators.

> Requires Portainer Business Edition 2.39+

## Example Usage

### Basic authentication
```hcl
resource "portainer_shared_git_credential" "example" {
  name               = "my-git-credential"
  username           = "git-user"
  password           = "git-password"
  authorization_type = 0
}
```

### Token authentication
```hcl
resource "portainer_shared_git_credential" "token_example" {
  name               = "my-git-token"
  username           = "git-user"
  password           = "ghp_xxxxxxxxxxxxxxxxxxxx"
  authorization_type = 1
}
```

## Lifecycle & Behavior
Shared git credentials are created via the `/cloud/gitcredentials` API.

Only administrators can create, update, or delete shared git credentials.

The `password` field is sensitive and will not be displayed in plan output.

- To destroy a shared git credential:
```hcl
terraform destroy
```

- To update a shared git credential, update the relevant fields and re-apply:
```hcl
terraform apply
```

## Arguments Reference
| **Name**             | **Type** | **Required** | **Description**                                           |
|----------------------|----------|--------------|-----------------------------------------------------------|
| `name`               | string   | yes          | Name of the shared git credential                         |
| `username`           | string   | yes          | Username for git authentication                           |
| `password`           | string   | yes          | Password or token for git authentication (sensitive)      |
| `authorization_type` | int      | no           | Authorization type: `0` = Basic (default), `1` = Token    |

## Attributes Reference
| Name      | Description                          |
|-----------|--------------------------------------|
| `id`      | ID of the shared git credential      |
| `user_id` | User ID of the credential owner      |

## Import

Shared git credentials can be imported using their ID:

```shell
terraform import portainer_shared_git_credential.example 123
```
