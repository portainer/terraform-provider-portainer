# Data Source Documentation: `portainer_docker_snapshot`

# portainer_docker_snapshot

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reads the latest snapshot Portainer took of a Docker environment.

## Example Usage

```hcl
data "portainer_docker_snapshot" "prod" {
  endpoint_id = portainer_environment.prod.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Identifier of the Docker environment whose snapshot is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `snapshot` | string | The latest snapshot as Portainer returns it, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_docker_snapshot_containers` for a typed view of the containers in it. |

Portainer's own API specification gives this response no fields at all, so it is carried through as JSON rather than flattened into attributes that could not stay accurate. Use `portainer_docker_snapshot_containers` for a typed view of the containers in it.

A snapshot is what Portainer last saw, not a live query, so it can lag behind the environment.
