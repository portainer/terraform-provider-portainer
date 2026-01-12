# 📟 **Data Source Documentation: `portainer_edge_stack`**

# portainer_edge_stack
The `portainer_edge_stack` data source allows you to look up an existing Portainer Edge stack by its name.

## Example Usage

### Look up an Edge stack by name

```hcl
data "portainer_edge_stack" "monitoring" {
  name = "edge-monitoring"
}

output "edge_stack_id" {
  value = data.portainer_edge_stack.monitoring.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                |
|--------|--------|----------|----------------------------|
| `name` | string | ✅ yes   | Name of the Portainer Edge stack. |

## Attributes Reference

| Name              | Type    | Description                                             |
|-------------------|---------|---------------------------------------------------------|
| `id`              | string  | ID of the Portainer Edge stack.                         |
| `deployment_type` | integer | Deployment type: `0`=Docker Compose, `1`=Kubernetes. |
