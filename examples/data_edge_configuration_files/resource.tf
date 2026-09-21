data "portainer_edge_configuration_files" "shops" {
  edge_configuration_id = 1
}

output "edge_configuration_files" {
  value = data.portainer_edge_configuration_files.shops.files
}
