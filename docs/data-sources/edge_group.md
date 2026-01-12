# 🛰️ **Data Source Documentation: `portainer_edge_group`**

# portainer_edge_group
The `portainer_edge_group` data source allows you to look up an existing Portainer Edge group by its name.

## Example Usage

### Look up an Edge group by name

```hcl
data "portainer_edge_group" "retail_stores" {
  name = "Retail-Stores"
}

output "edge_group_id" {
  value = data.portainer_edge_group.retail_stores.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                     |
|--------|--------|----------|---------------------------------|
| `name` | string | ✅ yes   | Name of the Portainer Edge group. |

## Attributes Reference

| Name      | Type | Description                          |
|-----------|------|--------------------------------------|
| `id`      | string | ID of the Portainer Edge group.      |
| `dynamic` | bool | Whether the group is dynamic or static. |
