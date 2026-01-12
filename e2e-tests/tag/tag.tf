resource "portainer_tag" "your-tag" {
  name = var.portainer_tag_name
}

data "portainer_tag" "test_lookup" {
  name = portainer_tag.your-tag.name
}

output "found_tag_id" {
  value = data.portainer_tag.test_lookup.id
}
