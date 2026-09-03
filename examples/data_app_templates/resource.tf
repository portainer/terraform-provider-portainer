data "portainer_app_templates" "catalogue" {}

output "available_templates" {
  value = [for t in data.portainer_app_templates.catalogue.templates : t.title]
}
