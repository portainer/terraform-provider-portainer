# Data Source Documentation: `portainer_custom_template_file`

# portainer_custom_template_file
Reads the stack file stored behind a custom template.

## Example Usage

```hcl
data "portainer_custom_template_file" "web" {
  template_id = portainer_custom_template.web.id
}

output "web_template_body" {
  value = data.portainer_custom_template_file.web.file_content
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `template_id` | number | ✅ yes | Identifier of the custom template whose stack file is read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `file_content` | string | Content of the stored file, exactly as Portainer holds it. |
