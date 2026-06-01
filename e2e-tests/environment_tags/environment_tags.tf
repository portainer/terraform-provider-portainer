resource "portainer_tag" "first" {
  name = "env-tag-alpha"
}

resource "portainer_tag" "second" {
  name = "env-tag-beta"
}

resource "portainer_environment" "with_tags" {
  name                = var.portainer_environment_name
  environment_address = var.portainer_environment_address
  type                = var.portainer_environment_type
  tls_enabled         = false
  tag_ids             = [portainer_tag.first.id, portainer_tag.second.id]
}
