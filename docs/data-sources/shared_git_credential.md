# Data Source Documentation: `portainer_shared_git_credential`

# portainer_shared_git_credential
The `portainer_shared_git_credential` data source allows you to look up existing shared git credentials by their name.

## Example Usage

### Look up shared git credentials by name

```hcl
data "portainer_shared_git_credential" "my_cred" {
  name = "my-git-credential"
}

output "credential_id" {
  value = data.portainer_shared_git_credential.my_cred.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                                  |
|--------|--------|----------|----------------------------------------------|
| `name` | string | yes      | Name of the shared git credential to look up |

## Attributes Reference

| Name                 | Type   | Description                                       |
|----------------------|--------|---------------------------------------------------|
| `id`                 | string | ID of the shared git credential                   |
| `username`           | string | Username for git authentication                   |
| `authorization_type` | int    | Authorization type: `0` = Basic, `1` = Token      |
| `user_id`            | int    | User ID of the credential owner                   |
