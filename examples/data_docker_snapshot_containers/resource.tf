data "portainer_docker_snapshot_containers" "prod" {
  endpoint_id = 1
}

output "docker_stopped_containers" {
  value = [
    for c in data.portainer_docker_snapshot_containers.prod.containers : c.names[0]
    if c.state != "running"
  ]
}
