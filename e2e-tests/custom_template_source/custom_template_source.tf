resource "portainer_gitops_source" "e2e" {
  name     = "e2e-template-source"
  url      = var.source_url
  interval = "1h"
  public   = true
}

resource "portainer_custom_template" "from_source" {
  title             = "e2e-template-from-source"
  description       = var.template_description
  note              = "e2e"
  platform          = 1
  type              = 2
  source_id         = portainer_gitops_source.e2e.id
  compose_file_path = "docker-compose.yml"
}

output "template_id" {
  value = portainer_custom_template.from_source.id
}
