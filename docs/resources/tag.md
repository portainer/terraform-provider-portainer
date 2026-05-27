# 🏷️ **Resource Documentation: `portainer_tag`**

# portainer_tag
The `portainer_tag` resource allows you to manage tags in Portainer.

## Example Usage

### Create Tag

```hcl
resource "portainer_tag" "your-tag" {
  name = "your-tag"
}
```
## Lifecycle & Behavior

Tags are recreated if their name changes.

- To delete a tag created via Terraform, simply run:
```hcl
terraform destroy
```

- To rename a tag, update the name and re-apply:
```hcl
terraform apply
```

## Arguments Reference

| Name        | Type    | Required                  | Description                                                                 |
|-------------|---------|---------------------------|-----------------------------------------------------------------------------|
| `name`      | string  | ✅ yes                    | 	Name of the Portainer tag.                                       |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer tag  |

## Import

Tags can be imported using their numeric ID:

```shell
terraform import portainer_tag.example 42
```
