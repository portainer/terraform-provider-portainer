# Data Source Documentation: `portainer_helm_git_dryrun`

# portainer_helm_git_dryrun
The `portainer_helm_git_dryrun` data source allows you to perform a dry-run validation of a Helm chart deployment from a Git repository before actually applying it. This is useful for previewing the rendered manifest and detecting configuration issues early.

## Example Usage

```hcl
data "portainer_helm_git_dryrun" "test" {
  endpoint_id    = 1
  repository_url = "https://github.com/my-org/helm-charts.git"
  reference_name = "refs/heads/main"
  chart_path     = "charts/my-app"
  release_name   = "my-app"
  namespace      = "default"
  values_files   = ["values.yaml", "values-prod.yaml"]
}

output "rendered_manifest" {
  value = data.portainer_helm_git_dryrun.test.manifest
}
```

## Arguments Reference

| Name                           | Type         | Required | Description                                                |
|--------------------------------|--------------|----------|------------------------------------------------------------|
| `endpoint_id`                  | int          | yes      | Environment (Endpoint) identifier.                         |
| `repository_url`               | string       | yes      | URL of the Git repository containing the Helm chart.       |
| `reference_name`               | string       | no       | Git reference name (e.g. refs/heads/main).                 |
| `chart_path`                   | string       | no       | Path to the Helm chart in the repository.                  |
| `values_files`                 | list(string) | no       | List of Helm values files to use.                          |
| `namespace`                    | string       | no       | Kubernetes namespace for the release.                      |
| `release_name`                 | string       | no       | Name of the Helm release.                                  |
| `repository_authentication`    | bool         | no       | Whether the repository requires authentication.            |
| `repository_username`          | string       | no       | Username for repository authentication.                    |
| `repository_password`          | string       | no       | Password for repository authentication (sensitive).        |
| `repository_git_credential_id` | int          | no       | Git credential ID for repository authentication.           |
| `tls_skip_verify`              | bool         | no       | Skip TLS verification when cloning the repository.         |

## Attributes Reference

| Name              | Type   | Description                          |
|-------------------|--------|--------------------------------------|
| `manifest`        | string | Rendered manifest from the dry run.  |
| `release_version` | int    | Version (revision) of the release.   |
