# Data Source Documentation: `portainer_docker_dashboard`

# portainer_docker_dashboard
Counts what a Docker environment holds: containers by state, images, volumes, networks, services and stacks — the same figures Portainer's own dashboard shows.

## Example Usage

```hcl
data "portainer_docker_dashboard" "prod" {
  environment_id = 1
}

output "unhealthy_containers" {
  value = data.portainer_docker_dashboard.prod.containers_unhealthy
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment (endpoint) identifier of the Docker environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `containers_total` | number | Total containers. |
| `containers_running` | number | Running containers. |
| `containers_stopped` | number | Stopped containers. |
| `containers_healthy` | number | Containers reporting healthy. |
| `containers_unhealthy` | number | Containers reporting unhealthy. |
| `images_total` | number | Images stored on the host. |
| `images_size` | number | Total size of those images in bytes. |
| `volumes` | number | Volumes in the environment. |
| `networks` | number | Networks in the environment. |
| `services` | number | Swarm services, zero on a standalone host. |
| `stacks` | number | Stacks in the environment. |

A container without a healthcheck counts towards neither `containers_healthy` nor `containers_unhealthy`.
