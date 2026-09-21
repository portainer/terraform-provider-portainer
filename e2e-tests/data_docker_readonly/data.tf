# Read-only Docker data sources against the Docker environment the suite
# registers. portainer_docker_container_gpus is absent: it needs a specific
# container ID and GPU hardware that CI does not have.

data "portainer_docker_dashboard" "host" {
  environment_id = var.endpoint_id
}

data "portainer_docker_images" "host" {
  environment_id = var.endpoint_id
  with_usage     = true
}

output "container_total" {
  value = data.portainer_docker_dashboard.host.containers_total

  # Portainer itself runs as a container on this host.
  precondition {
    condition     = data.portainer_docker_dashboard.host.containers_total > 0
    error_message = "the dashboard must count at least the Portainer container."
  }
}

output "image_count" {
  value = length(data.portainer_docker_images.host.images)
}

output "image_total_size" {
  value = data.portainer_docker_images.host.total_size
}

output "unused_images" {
  value = data.portainer_docker_images.host.unused_count
}
