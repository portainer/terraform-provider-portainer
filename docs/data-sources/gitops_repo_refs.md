# Data Source Documentation: `portainer_gitops_repo_refs`

# portainer_gitops_repo_refs
The `portainer_gitops_repo_refs` data source lists all branches and tags (refs) from a Git repository.

## Example Usage

```hcl
data "portainer_gitops_repo_refs" "example" {
  repository_url = "https://github.com/portainer/portainer.git"
}

output "git_refs" {
  value = data.portainer_gitops_repo_refs.example.refs
}
```

### With authentication
```hcl
data "portainer_gitops_repo_refs" "private" {
  repository_url = "https://github.com/my-org/private-repo.git"
  username       = "my-user"
  password       = var.git_password
}
```

## Arguments Reference

| Name              | Type   | Required | Description                                          |
|-------------------|--------|----------|------------------------------------------------------|
| `repository_url`  | string | yes      | URL of the Git repository.                           |
| `username`        | string | no       | Username for repository authentication.              |
| `password`        | string | no       | Password for repository authentication (sensitive).  |
| `git_credential_id` | int | no       | Git credential ID for authentication.                |
| `tls_skip_verify` | bool   | no       | Skip TLS verification when cloning the repository.   |

## Attributes Reference

| Name   | Type         | Description                              |
|--------|--------------|------------------------------------------|
| `refs` | list(string) | List of Git references (branches/tags).  |
