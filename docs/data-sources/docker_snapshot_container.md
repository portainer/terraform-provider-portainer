# Data Source Documentation: `portainer_docker_snapshot_container`

# portainer_docker_snapshot_container

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reads one container from the latest snapshot of a Docker environment.

## Example Usage

```hcl
data "portainer_docker_snapshot_container" "web" {
  endpoint_id  = portainer_environment.prod.id
  container_id = "abc123"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `container_id` | string | ✅ yes | Identifier of the container to read from the snapshot. |
| `endpoint_id` | number | ✅ yes | Identifier of the Docker environment whose snapshot is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `command` | string | Command the container runs. |
| `created` | number | Unix timestamp the container was created at. |
| `details` | string | The whole snapshot entry as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the mounts, networks and host configuration the attributes above do not cover. |
| `image` | string | Image the container runs. |
| `image_id` | string | Identifier of that image. |
| `labels` | map(string) | Labels on the container. |
| `names` | list(string) | Names of the container, as Docker reports them with a leading slash. |
| `state` | string | State of the container, for example `running` or `exited`. |
| `status` | string | Human-readable status, for example `Up 3 hours`. |

Mounts, network settings and the host configuration follow Docker's own schema rather than Portainer's and nest deeply, so `details` carries the whole entry as JSON alongside the attributes above.
