data "portainer_endpoint_registries" "prod" {
  environment_id        = var.environment_id
  dockerhub_registry_id = 6
}

# Do not start a large rollout on the last few pulls of the window.
output "dockerhub_headroom" {
  value = "${data.portainer_endpoint_registries.prod.dockerhub_rate_remaining} of ${data.portainer_endpoint_registries.prod.dockerhub_rate_limit}"
}
