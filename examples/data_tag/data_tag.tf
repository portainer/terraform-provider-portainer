data "portainer_tag" "example" {
  name = var.tag_name
}

output "tag_id" {
  value = data.portainer_tag.example.id
}

output "tag_name" {
  value = data.portainer_tag.example.name
}
