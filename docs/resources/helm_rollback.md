# Resource Documentation: `portainer_helm_rollback`

# portainer_helm_rollback
The `portainer_helm_rollback` resource triggers a rollback of a Helm release to a previous revision. This is an action-style resource: it performs the rollback on create and has no read or delete side effects.

## Example Usage

```hcl
resource "portainer_helm_rollback" "rollback" {
  endpoint_id  = 1
  release_name = "my-release"
  namespace    = "default"
  revision     = 3
}
```

- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/helm_rollback)

## Lifecycle & Behavior
This resource performs a one-time Helm rollback when applied. It does not manage ongoing state and will re-trigger on each `terraform apply`.

## Arguments Reference

| Name            | Type   | Required | Description                                                                        |
|-----------------|--------|----------|------------------------------------------------------------------------------------|
| `endpoint_id`   | int    | yes      | Environment (Endpoint) identifier.                                                 |
| `release_name`  | string | yes      | Name of the Helm release to rollback.                                              |
| `namespace`     | string | no       | Kubernetes namespace of the release.                                               |
| `revision`      | int    | no       | Revision number to rollback to (defaults to previous revision if not specified).    |
| `wait`          | bool   | no       | Wait for resources to be ready (default: false).                                   |
| `wait_for_jobs` | bool   | no       | Wait for jobs to complete (default: false).                                        |
| `recreate`      | bool   | no       | Restart pods for the resource if applicable (default: true).                       |
| `force`         | bool   | no       | Force resource update through delete/recreate if needed (default: false).          |
| `timeout`       | int    | no       | Time to wait for any individual Kubernetes operation in seconds (default: 300).     |
