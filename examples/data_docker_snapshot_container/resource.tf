data "portainer_docker_snapshot_container" "web" {
  endpoint_id  = 1
  container_id = "abc123"
}

output "docker_container_status" {
  value = data.portainer_docker_snapshot_container.web.status
}
