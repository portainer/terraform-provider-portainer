data "portainer_custom_template" "example" {
  title = var.custom_template_title
}

output "custom_template_id" {
  value = data.portainer_custom_template.example.id
}

output "custom_template_description" {
  value = data.portainer_custom_template.example.description
}

output "custom_template_type" {
  value = data.portainer_custom_template.example.type
}
