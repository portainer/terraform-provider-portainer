# 🔐 **Resource Documentation: `portainer_resource_control`**

# portainer_resource_control
The `portainer_resource_control` resource allows you to manage access control for Docker resources within Portainer.
You can assign access permissions to specific users or teams, or make the resource public or admin-only.

## Example Usage
```hcl
resource "portainer_resource_control" "example" {
  resource_id          = "617c5f22bb9b023d6daab7cba43a57576f83492867bc767d1c59416b065e5f08"
  type                 = 1
  administrators_only  = true
  public               = false
  sub_resource_ids     = []
  teams                = [7]
  users                = [4]
}
```

## Lifecycle & Behavior
- If any permission fields (teams, users, administrators_only, or public) are changed, the resource is updated in place by re-run:
```hcl
terraform apply
```
- Changes to resource_id, sub_resource_ids, or type require recreation of the resource.
- For destroy some resource_control run:
```hcl
terraform destroy
```

### Arguments Reference
| Name               | Type           | Required | Description                                                                 |
|--------------------|----------------|----------|-----------------------------------------------------------------------------|
| `resource_id`      | string         | ✅ yes   | Unique ID of the resource to control (e.g., container or service ID).      |
| `type`             | number         | ✅ yes   | Type of the resource (`1` = container, `2` = service, `3` = volume, etc.). |
| `administrators_only` | bool       | 🚫 no    | Restrict access to administrators only. Default: `false`.                  |
| `public`           | bool           | 🚫 no    | Make resource public to all users. Default: `false`.                       |
| `sub_resource_ids` | list(string)   | 🚫 no    | List of sub-resource IDs, e.g. container IDs within a stack.              |
| `teams`            | list(number)   | 🚫 no    | List of team IDs with access.                                              |
| `users`            | list(number)   | 🚫 no    | List of user IDs with access.                                              |

---

### Attributes Reference

| Name | Description                               |
|------|-------------------------------------------|
| `id` | ID of the resource control in Portainer   |
