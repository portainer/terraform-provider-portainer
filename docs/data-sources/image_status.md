# Data Source Documentation: `portainer_image_status`

# portainer_image_status

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reports whether the image behind a container, a service or a whole stack has been superseded in its registry.

## Example Usage

```hcl
data "portainer_image_status" "web" {
  kind        = "container"
  endpoint_id = portainer_environment.prod.id
  resource_id = "abc123"
}

data "portainer_image_status" "stack" {
  kind        = "stack"
  resource_id = portainer_stack.web.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ❌ no | Identifier of the environment. Required for `container` and `service`; a stack is addressed by its own identifier instead. |
| `kind` | string | ✅ yes | What to check: `container`, `service` or `stack`. |
| `refresh` | bool | ❌ no | Whether to re-check the registry rather than answer from Portainer's cache. A refresh costs a registry round trip per image. |
| `resource_id` | string | ✅ yes | Identifier of the container, service or stack to check. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `message` | string | Explanation of the status, empty when there is nothing to say. |
| `status` | string | What Portainer found: `updated`, `outdated`, `skipped`, `processing`, `preparing` or `error`. Only `outdated` means a newer image is published. |

Portainer has three endpoints for this, one per subject, which is what `kind` selects. `endpoint_id` is required for `container` and `service`; a stack is addressed by its own identifier and has no environment in its path.

Only `outdated` means a newer image is published. `refresh` re-checks the registry instead of answering from Portainer's cache, at the cost of a registry round trip per image.
