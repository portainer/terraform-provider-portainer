# 📦 **Data Source Documentation: `portainer_registry`**

# portainer_registry
The `portainer_registry` data source allows you to look up an existing Portainer registry by its name.

## Example Usage

### Look up a registry by name

```hcl
data "portainer_registry" "dockerhub" {
  name = "DockerHub"
}

output "registry_url" {
  value = data.portainer_registry.dockerhub.url
}
```

## Arguments Reference

| Name   | Type   | Required | Description               |
|--------|--------|----------|---------------------------|
| `name` | string | ✅ yes   | Name of the Portainer registry. |

## Attributes Reference

| Name   | Type    | Description                                             |
|--------|---------|---------------------------------------------------------|
| `id`   | string  | ID of the Portainer registry.                           |
| `url`  | string  | URL of the registry.                                    |
| `type` | integer | Type of registry (e.g., `1`=Quay, `3`=Custom, `6`=DH). |
