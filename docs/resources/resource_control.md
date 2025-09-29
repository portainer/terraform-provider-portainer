# 🔐 **Resource Documentation: `portainer_resource_control`**

## portainer_resource_control

The `portainer_resource_control` resource allows you to **update** access control (AccessPolicy) for existing Docker stacks within Portainer.
You can assign access permissions to specific users or teams, or make the stack public or admin-only.

⚠️ **Important limitations:**

* `create` does not create a new AccessPolicy. It internally calls `update` and **assumes that the AccessPolicy already exists** (Portainer automatically creates it when you create a stack).
* Currently, only **stacks** (`type = 6`) are supported. Other resource types (images, networks, Helm releases, etc.) are not yet implemented.

---

## Example Usage

```hcl
resource "portainer_stack" "standalone_file" {
  name            = "my-stack"
  deployment_type = "standalone"
  method          = "file"
  endpoint_id     = 3

  stack_file_path = "${path.module}/docker-compose.yml"
}

resource "portainer_resource_control" "stack_access" {
  resource_id          = portainer_stack.standalone_file.id
  type                 = 6
  administrators_only  = false
  public               = false
  teams                = [8]
  users                = []
}
```
---

## Lifecycle & Behavior

* **Create**: does not create a new AccessPolicy. Instead, it calls update on the existing AccessPolicy that Portainer created automatically with the stack.
* **Update**: if you change permission fields (`teams`, `users`, `administrators_only`, or `public`), Terraform applies the changes to the existing AccessPolicy.
* **Delete**: removes the AccessPolicy associated with the stack.
* Changes to `resource_id` or `type` force a new resource.

---

### Arguments Reference

| Name                  | Type         | Required | Description                                                      |
| --------------------- | ------------ | -------- | ---------------------------------------------------------------- |
| `resource_id`         | string       | ✅ yes    | ID of the stack in Portainer (e.g. `46`).                       |
| `type`                | number       | ✅ yes    | Type of the resource. Currently only `6` (= stack) is supported.|
| `administrators_only` | bool         | 🚫 no    | Restrict access to administrators only. Default: `false`.        |
| `public`              | bool         | 🚫 no    | Make resource public to all users. Default: `false`.             |
| `teams`               | list(number) | 🚫 no    | List of team IDs with access.                                    |
| `users`               | list(number) | 🚫 no    | List of user IDs with access.                                    |

---

### Attributes Reference

| Name | Description                                             |
| ---- | ------------------------------------------------------- |
| `id` | ID of the access policy (resource control) in Portainer |
