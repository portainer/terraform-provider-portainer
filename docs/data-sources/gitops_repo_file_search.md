# Data Source Documentation: `portainer_gitops_repo_file_search`

# portainer_gitops_repo_file_search

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Searches a git repository for file paths, which is how a configuration discovers what it can deploy.

## Example Usage

```hcl
data "portainer_gitops_repo_file_search" "stacks" {
  repository = "https://github.com/example/platform"
  reference  = "refs/heads/main"
  include    = "yml,yaml"
  keyword    = "portainer"
}

output "deployable_files" {
  value = data.portainer_gitops_repo_file_search.stacks.paths
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `directories_only` | bool | ❌ no | Whether to return only directories rather than files. |
| `force` | bool | ❌ no | Whether to re-clone the repository rather than answer from Portainer's cache. |
| `include` | string | ❌ no | File extensions to include, for example `yml,yaml`. |
| `keyword` | string | ❌ no | Only return paths matching this keyword. |
| `password` | string | ❌ no | Password or token to authenticate with. Stored in state as a sensitive value. |
| `reference` | string | ✅ yes | Git reference to search at, for example `refs/heads/main`. |
| `repository` | string | ✅ yes | URL of the git repository to search. |
| `source_id` | number | ❌ no | Identifier of a GitOps source to take the credentials from instead of giving them here. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the git server's TLS certificate. |
| `username` | string | ❌ no | Username to authenticate to the repository with. Leave unset for a public repository or when `source_id` supplies the credentials. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `paths` | list(string) | The paths the search matched, sorted. |

The credentials are omitted from the request when unset, so a public repository is never cloned with a blank username and password. Supply `source_id` instead to reuse a GitOps source's credentials.

`force` re-clones rather than answering from Portainer's cache. The returned paths are sorted so two identical plans produce identical output.
