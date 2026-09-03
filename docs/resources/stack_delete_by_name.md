# Resource Documentation: `portainer_stack_delete_by_name`

# portainer_stack_delete_by_name
The `portainer_stack_delete_by_name` resource removes Kubernetes stacks by name, which is the only way to clear a stack Terraform does not have an ID for — typically one deployed outside Portainer and only discovered by it.

This is an **action resource**: the work happens when it is created, and destroying it does nothing. It is not a way to manage a stack's lifecycle; use `portainer_stack` for stacks Terraform owns. Reach for this only to clean up something Terraform never created, usually before adopting a namespace.

## Example Usage

```hcl
resource "portainer_stack_delete_by_name" "legacy" {
  name        = "legacy-app"
  endpoint_id = 4
  external    = true
}
```

## Arguments Reference

| Name          | Type   | Required | Default | Description                                                                                            |
|---------------|--------|----------|---------|------------------------------------------------------------------------------------------------------|
| `name`        | string | ✅ yes   | –       | Name of the stack to remove. **Every** stack with this name in the environment is removed.               |
| `endpoint_id` | number | ✅ yes   | –       | Identifier of the environment the stack was deployed to.                                                 |
| `external`    | bool   | ❌ no    | `false` | Whether the stack was created outside Portainer, rather than deployed by it.                             |

All arguments force a new resource.

## Attributes Reference

| Name | Description                                                                  |
|------|------------------------------------------------------------------------------|
| `id` | Synthetic identifier in the form `<endpoint_id>/<name>/deleted/<timestamp>`.  |

A stack that is already gone is treated as success — that is the state this resource asks for.
