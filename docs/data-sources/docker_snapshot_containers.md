# Data Source Documentation: `portainer_docker_snapshot_containers`

# portainer_docker_snapshot_containers

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the containers in the latest snapshot of a Docker environment.

## Example Usage

```hcl
data "portainer_docker_snapshot_containers" "prod" {
  endpoint_id = portainer_environment.prod.id
}

output "stopped_containers" {
  value = [for c in data.portainer_docker_snapshot_containers.prod.containers : c.names[0] if c.state != "running"]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_stack_id` | number | ❌ no | Only list the containers belonging to this edge stack. |
| `endpoint_id` | number | ✅ yes | Identifier of the Docker environment whose snapshot is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `containers` | list(object) | The containers in the latest snapshot. This is what Portainer last saw, not a live query, so it can lag behind the environment. |

### `containers`

| Name | Type | Description |
|------|------|-------------|
| `command` | string | Command the container runs. |
| `created` | number | Unix timestamp the container was created at. |
| `id` | string | Container identifier. |
| `image` | string | Image the container runs. |
| `image_id` | string | Identifier of that image. |
| `labels` | map(string) | Labels on the container. |
| `names` | list(string) | Names of the container, as Docker reports them with a leading slash. |
| `ports` | list(object) | Ports the container exposes. |
| `state` | string | State of the container, for example `running` or `exited`. |
| `status` | string | Human-readable status, for example `Up 3 hours`. |

### `containers.ports`

| Name | Type | Description |
|------|------|-------------|
| `ip` | string | Host address the port is bound to. |
| `private_port` | number | Port inside the container. |
| `public_port` | number | Port on the host, zero when the port is not published. |
| `type` | string | Protocol of the port. |

Portainer's specification types this response as a single container even though the endpoint lists them. The provider accepts both shapes rather than trusting the document over the endpoint's own name.

A snapshot is what Portainer last saw, not a live query, so it can lag behind the environment.
