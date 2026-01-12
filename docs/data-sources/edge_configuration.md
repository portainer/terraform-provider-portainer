# ⚙️ **Data Source Documentation: `portainer_edge_configuration`**

# portainer_edge_configuration
The `portainer_edge_configuration` data source allows you to look up an existing Portainer Edge configuration by its name.

## Example Usage

### Look up an Edge configuration by name

```hcl
data "portainer_edge_configuration" "base_config" {
  name = "base-security-policy"
}

output "config_id" {
  value = data.portainer_edge_configuration.base_config.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                        |
|--------|--------|----------|------------------------------------|
| `name` | string | ✅ yes   | Name of the Edge configuration. |

## Attributes Reference

| Name       | Type    | Description                               |
|------------|---------|-------------------------------------------|
| `id`       | string  | ID of the Portainer Edge configuration. |
| `type`     | integer | Configuration type.                       |
| `category` | string  | Configuration category.                   |
