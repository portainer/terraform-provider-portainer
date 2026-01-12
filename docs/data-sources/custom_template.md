# 📝 **Data Source Documentation: `portainer_custom_template`**

# portainer_custom_template
The `portainer_custom_template` data source allows you to look up an existing Portainer custom template by its title.

## Example Usage

### Look up a custom template by title

```hcl
data "portainer_custom_template" "web_app" {
  title = "Standard Web App"
}

output "template_id" {
  value = data.portainer_custom_template.web_app.id
}
```

## Arguments Reference

| Name    | Type   | Required | Description                           |
|---------|--------|----------|---------------------------------------|
| `title` | string | ✅ yes   | Title of the Portainer custom template. |

## Attributes Reference

| Name          | Type    | Description                          |
|---------------|---------|--------------------------------------|
| `id`          | string  | ID of the Portainer custom template. |
| `description` | string  | Description of the template.         |
| `type`        | integer | Template type (Compose/Swarm/K8s).   |
