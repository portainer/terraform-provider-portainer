# 📦 **Resource Documentation: `portainer_endpoint_group`**

# portainer_endpoint_group
The `portainer_endpoint_group` resource allows you to manage environment (endpoint) groups in Portainer.

## Example Usage

### Create Endpoint Group
```hcl
resource "portainer_endpoint_group" "your-group" {
  name        = "your-group"
  description = "Description for your group"
}
```

### Create Endpoint Group

```hcl
resource "portainer_tag" "your-tag" {
  name = "your-tag"
}

resource "portainer_endpoint_group" "your-group" {
  name        = "Your group"
  description = "Group for something"
  tag_ids     = [portainer_tag.your-group.id]
}
```

## Lifecycle & Behavior

Endpoint groups are updated if any attributes change (e.g. name, description, tag_ids).

- To delete a group created via Terraform, simply run:
```hcl
terraform destroy
```

- To update name or tags, modify the fields and re-apply:
```hcl
terraform apply
```
> ⚠️ Portainer does not support in-place updates for endpoint groups via API. All changes will recreate the group.

## Arguments Reference

| Name          | Type       | Required     | Description                                                    |
|---------------|------------|--------------|----------------------------------------------------------------|
| `name`        | string     | ✅ yes       | Name of the Portainer endpoint group.                          |
| `description` | string     | 🚫 optional  | Optional description of the group.                             |
| `tag_ids`     | list(int)  | 🚫 optional  | List of Portainer tag IDs to associate with this group.        |                                     |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer endpoint group |
