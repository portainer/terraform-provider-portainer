resource "portainer_endpoint_group" "production" {
  name = "production"
}

resource "portainer_endpoint_group_membership" "prod_docker" {
  endpoint_group_id = portainer_endpoint_group.production.id
  endpoint_id       = 9
}
