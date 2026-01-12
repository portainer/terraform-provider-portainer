# 📂 **Data Source Documentation: `portainer_endpoint_group`**

# portainer_endpoint_group
The `portainer_endpoint_group` data source allows you to look up an existing Portainer endpoint group by its name.

## Example Usage

### Look up an endpoint group by name

```hcl
data "portainer_endpoint_group" "production" {
  name = "Production"
}

output "group_id" {
  value = data.portainer_endpoint_group.production.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                     |
|--------|--------|----------|---------------------------------|
| `name` | string | ✅ yes   | Name of the Portainer endpoint group. |

## Attributes Reference

| Name          | Type   | Description                          |
|---------------|--------|--------------------------------------|
| `id`          | string | ID of the Portainer endpoint group.  |
| `description` | string | Description of the endpoint group.   |
