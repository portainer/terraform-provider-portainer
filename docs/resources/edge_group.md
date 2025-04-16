# 👥🌐 **Resource Documentation: `portainer_edge_group`**

# portainer_edge_group
The `portainer_edge_group` resource allows you to manage Edge Groups in Portainer.
You can create both static and dynamic edge groups using this resource.

## Example Usage

### Create Static Edge Group

```hcl
resource "portainer_edge_group" "static_group" {
  name    = "static-group"
  dynamic = false
}
```

### Create Dynamic Edge Group (via tags)
```hcl
resource "portainer_tag" "your-tag" {
  name = "your-tag"
}

resource "portainer_edge_group" "dynamic_group" {
  name           = "dynamic-group"
  dynamic        = true
  partial_match  = true
  tag_ids        = [portainer_tag.your-group.id]
}
```
## Lifecycle & Behavior
Edge groups are updated if any of the attributes change (e.g., name, tag_ids, endpoints, etc.).
- To delete an edge group created via Terraform, simply run:
```hcl
terraform destroy
```

- To modify a group (e.g., make it dynamic), update the attributes and re-apply:
```hcl
terraform apply
```

## Arguments Reference

| Name            | Type        | Required       | Description                                                                 |
|-----------------|-------------|----------------|-----------------------------------------------------------------------------|
| `name`          | string      | ✅ yes         | Name of the Edge Group.                                                     |
| `dynamic`       | bool        | ✅ yes         | If true, the group is dynamic (matched by tags); if false, it's static.     |
| `partial_match` | bool        | 🚫 optional    | For dynamic groups, if true, partial match on tags is used. Default: false. |
| `tag_ids`       | list(int)   | 🚫 optional    | List of tag IDs to use for matching environments in dynamic groups.         |
| `endpoints`     | list(int)   | 🚫 optional    | List of environment IDs to assign manually (for static groups).             |
> ⚠️ When dynamic = true, you should provide tag_ids.
> ⚠️ When dynamic = false, you may provide endpoints.

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the Portainer edge group |
