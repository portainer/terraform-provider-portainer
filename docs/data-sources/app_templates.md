# Data Source Documentation: `portainer_app_templates`

# portainer_app_templates
Lists Portainer's application template catalogue, and optionally fetches the stack file of one template.

## Example Usage

```hcl
data "portainer_app_templates" "catalogue" {}

output "available_templates" {
  value = [for t in data.portainer_app_templates.catalogue.templates : t.title]
}

# Deploy a stack straight from a catalogue template.
data "portainer_app_templates" "wordpress" {
  template_id = 12
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `template_id` | number | ❌ no | Template whose stack file should also be fetched. Leave unset to only list the catalogue. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `version` | string | Version of the catalogue Portainer is serving. |
| `templates` | list | Templates in the catalogue. Each entry has `id`, `title`, `description`, `type`, `platform`, `categories`, `image` and `repository_url`. |
| `file_content` | string | Stack file of the template named by `template_id`, empty when that argument is unset. |

Portainer models the file endpoint as a `POST` even though it only reads, which is why fetching a template's file has no side effect despite the verb.
