# 🔗 **Resource Documentation: `portainer_endpoint_association`**

# portainer_endpoint_association
The `portainer_endpoint_association` resource allows you to de-associate an Edge environment (endpoint) from Portainer.

This operation is useful when you want to disconnect an Edge agent from Portainer but keep the environment record.
## Example Usage
```hcl
resource "portainer_endpoint_association" "example" {
  endpoint_id = 3
}
```

## Lifecycle & Behavior
For de-association of the Edge environment run:
```hcl
trraform apply
```

## Arguments Reference
| Name         | Type   | Required | Description                                         |
|--------------|--------|----------|-----------------------------------------------------|
| `endpoint_id`| number | ✅ yes   | ID of the environment (endpoint) to de-associate    |

## Attributes Reference

| Name | Description                                      |
|------|--------------------------------------------------|
| `id` | Same as `endpoint_id` used in input.             |
