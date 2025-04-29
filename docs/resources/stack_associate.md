# 🧩🗑️ **Resource Documentation: `portainer_stack_associate`**

## `portainer_stack_associate`

The `portainer_stack_associate` resource allows you to associate an **orphaned stack** to a new environment (endpoint) within Portainer. This is useful when you manually deploy a stack and want to register it under a specific environment in Portainer for further management.

---

## ✅ Example Usage

```hcl
resource "portainer_stack_associate" "example" {
  stack_id         = 12
  endpoint_id      = 1
  swarm_id         = "jpofkc0i9uo9wtx1zesuk649w"
  orphaned_running = true
}
```

## Lifecycle & Behavior
For de-association of the Stack in docker swarm run:
```hcl
trraform apply
```

## Arguments Reference
| Name             | Type    | Required | Description                                                             |
|------------------|---------|----------|-------------------------------------------------------------------------|
| `stack_id`       | number  | ✅ yes   | ID of the orphaned stack to associate.                                  |
| `endpoint_id`    | number  | ✅ yes   | ID of the environment (endpoint) to associate the stack with.           |
| `swarm_id`       | string  | ✅ yes   | Swarm cluster ID the stack should be associated with.                   |
| `orphaned_running` | boolean | optional | Whether the stack is orphaned and already running (default: `false`). |

## Attributes Reference

| Name | Description                                      |
|------|--------------------------------------------------|
| `id` | Same as `stack_id` used in input.                |