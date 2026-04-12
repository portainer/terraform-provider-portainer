data "portainer_policy_template" "example" {
  name = var.policy_template_name
}

output "template_id" {
  value = data.portainer_policy_template.example.id
}

output "template_description" {
  value = data.portainer_policy_template.example.description
}

output "template_category" {
  value = data.portainer_policy_template.example.category
}
