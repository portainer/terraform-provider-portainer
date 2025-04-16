resource "portainer_docker_image" "image_test" {
  endpoint_id = var.endpoint_id
  image       = var.image
}
