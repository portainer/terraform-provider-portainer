# Data Source Documentation: `portainer_gitops_helm_values`

# portainer_gitops_helm_values

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Previews the Helm values a deployment would receive, by merging values files from a git repository.

## Example Usage

```hcl
data "portainer_gitops_helm_values" "monitoring" {
  repository = "https://github.com/example/platform"
  reference  = "refs/heads/main"

  values_files = [
    "monitoring/values.yaml",
    "monitoring/values-prod.yaml",
  ]
}

output "merged_values" {
  value = data.portainer_gitops_helm_values.monitoring.merged_values
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `password` | string | ❌ no | Password or token to authenticate with. Stored in state as a sensitive value. |
| `reference` | string | ✅ yes | Git reference to read at, for example `refs/heads/main`. |
| `repository` | string | ✅ yes | URL of the git repository holding the values files. |
| `source_id` | number | ❌ no | Identifier of a GitOps source to take the credentials from instead of giving them here. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the git server's TLS certificate. |
| `username` | string | ❌ no | Username to authenticate to the repository with. Leave unset for a public repository or when `source_id` supplies the credentials. |
| `values_files` | list(string) | ✅ yes | Paths of the values files to merge, in order. Later files win, the same way `helm -f` treats them. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `commit_hash` | string | Commit the values were read at. Watch this to notice the repository moving under a configuration. |
| `files_processed` | list(string) | The files that were actually merged, in the order they were applied. A file missing from this list was not found in the repository. |
| `merged_values` | string | The merged values as YAML, which is what a Helm deployment from these files would receive. |

`values_files` is applied in order and later files win, the same way `helm -f` treats them. `files_processed` is reported in the order Portainer actually merged them and is deliberately **not** sorted, because that order is what decides which value wins - a file missing from it was not found in the repository.

Watch `commit_hash` to notice the repository moving under a configuration.
