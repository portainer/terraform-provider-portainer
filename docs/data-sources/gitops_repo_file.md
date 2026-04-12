# Data Source Documentation: `portainer_gitops_repo_file`

# portainer_gitops_repo_file
The `portainer_gitops_repo_file` data source reads a file from a Git repository and returns its content. This is useful for previewing compose files, configuration files, or any other file stored in Git before deploying.

## Example Usage

```hcl
data "portainer_gitops_repo_file" "compose" {
  repository_url = "https://github.com/portainer/portainer.git"
  reference      = "refs/heads/main"
  target_file    = "docker-compose.yml"
}

output "compose_content" {
  value = data.portainer_gitops_repo_file.compose.file_content
}
```

### With authentication
```hcl
data "portainer_gitops_repo_file" "private_file" {
  repository_url = "https://github.com/my-org/private-repo.git"
  reference      = "refs/heads/main"
  target_file    = "deploy/values.yaml"
  username       = "my-user"
  password       = var.git_password
}
```

## Arguments Reference

| Name              | Type   | Required | Description                                          |
|-------------------|--------|----------|------------------------------------------------------|
| `repository_url`  | string | yes      | URL of the Git repository.                           |
| `reference`       | string | no       | Git reference (e.g. refs/heads/master).              |
| `target_file`     | string | no       | Path to the file to read.                            |
| `username`        | string | no       | Username for repository authentication.              |
| `password`        | string | no       | Password for repository authentication (sensitive).  |
| `git_credential_id` | int | no       | Git credential ID for authentication.                |
| `tls_skip_verify` | bool   | no       | Skip TLS verification when cloning the repository.   |

## Attributes Reference

| Name           | Type   | Description                                  |
|----------------|--------|----------------------------------------------|
| `file_content` | string | Content of the file from the Git repository. |
