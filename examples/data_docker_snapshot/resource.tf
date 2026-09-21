data "portainer_docker_snapshot" "prod" {
  endpoint_id = 1
}

output "docker_snapshot_json" {
  value = data.portainer_docker_snapshot.prod.snapshot
}
