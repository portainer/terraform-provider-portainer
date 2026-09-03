# Data Source Documentation: `portainer_endpoint_registries`

# portainer_endpoint_registries
Lists the registries a given environment can pull from, and optionally the Docker Hub pull rate limit that applies to it.

## Example Usage

```hcl
data "portainer_endpoint_registries" "prod" {
  environment_id        = 1
  dockerhub_registry_id = 6
}

# Do not start a large rollout on the last few pulls of the window.
output "dockerhub_headroom" {
  value = "${data.portainer_endpoint_registries.prod.dockerhub_rate_remaining} of ${data.portainer_endpoint_registries.prod.dockerhub_rate_limit}"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | Environment whose accessible registries are listed. |
| `namespace` | string | ❌ no | Kubernetes namespace to scope the list to, where registry access is granted per namespace. |
| `dockerhub_registry_id` | number | ❌ no | Docker Hub registry whose pull rate limit should also be fetched. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `registries` | list | Registries the environment can pull from. Each entry has `id`, `name`, `url`, `base_url`, `type`, `authentication` and `username`. |
| `dockerhub_rate_limit` | number | Docker Hub pull rate limit, zero when `dockerhub_registry_id` is unset. |
| `dockerhub_rate_remaining` | number | Pulls still available in the current window. |

**Credentials are deliberately not exposed.** Portainer's API returns the full registry object, including its password and any access token; this data source maps neither, because copying registry credentials into every state file that reads it would be a liability and nothing in Terraform needs them.
