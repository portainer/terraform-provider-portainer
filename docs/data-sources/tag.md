# 🏷️ **Data Source Documentation: `portainer_tag`**

# portainer_tag
The `portainer_tag` data source allows you to look up an existing Portainer tag by its name.

## Example Usage

### Look up a tag by name

```hcl
data "portainer_tag" "production" {
  name = "production"
}

resource "portainer_environment" "prod" {
  name                = "Production Env"
  environment_address = "tcp://example.com:2375"
  type                = 1
  tag_ids             = [data.portainer_tag.production.id]
}
```

## Arguments Reference

| Name   | Type   | Required | Description            |
|--------|--------|----------|------------------------|
| `name` | string | ✅ yes   | Name of the Portainer tag. |

## Attributes Reference

| Name | Type   | Description                |
|------|--------|----------------------------|
| `id` | string | ID of the Portainer tag.   |
