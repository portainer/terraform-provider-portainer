# Data Source Documentation: `portainer_helm_release_history`

# portainer_helm_release_history
The `portainer_helm_release_history` data source returns the revision history of a Helm release. This is useful for listing available revisions before performing a rollback or for auditing purposes.

## Example Usage

```hcl
data "portainer_helm_release_history" "history" {
  endpoint_id  = 1
  release_name = "my-release"
  namespace    = "default"
}

output "release_revisions" {
  value = data.portainer_helm_release_history.history.revisions
}
```

## Arguments Reference

| Name           | Type   | Required | Description                            |
|----------------|--------|----------|----------------------------------------|
| `endpoint_id`  | int    | yes      | Environment (Endpoint) identifier.     |
| `release_name` | string | yes      | Name of the Helm release.              |
| `namespace`    | string | no       | Kubernetes namespace of the release.   |

## Attributes Reference

| Name        | Type         | Description                  |
|-------------|--------------|------------------------------|
| `revisions` | list(object) | List of release revisions.   |

### Revision object attributes

| Name          | Type   | Description                                        |
|---------------|--------|----------------------------------------------------|
| `revision`    | int    | Revision number.                                   |
| `status`      | string | Status of the revision (e.g. deployed, superseded). |
| `chart`       | string | Chart name and version.                             |
| `app_version` | string | Application version.                                |
| `description` | string | Description of the revision.                        |
| `updated`     | string | Timestamp when this revision was last deployed.     |
| `name`        | string | Name of the release.                                |
| `namespace`   | string | Namespace of the release.                           |
