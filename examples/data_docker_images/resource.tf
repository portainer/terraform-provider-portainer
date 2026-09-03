data "portainer_docker_images" "prod" {
  environment_id = var.environment_id
  with_usage     = true
}

output "reclaimable_bytes" {
  value = sum([for i in data.portainer_docker_images.prod.images : i.size if !i.used])
}
