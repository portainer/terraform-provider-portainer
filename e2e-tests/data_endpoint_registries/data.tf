# The suite registers no registries, so this exercises the empty-list path -
# which is the one that would break on a nil-vs-empty mistake. The Docker Hub
# rate limit lookup is left out: it needs a registry to ask about.

data "portainer_endpoint_registries" "host" {
  environment_id = var.endpoint_id
}

output "registry_count" {
  value = length(data.portainer_endpoint_registries.host.registries)
}

output "dockerhub_limit_not_requested" {
  value = data.portainer_endpoint_registries.host.dockerhub_rate_limit

  # Omitting dockerhub_registry_id must skip that lookup entirely.
  precondition {
    condition     = data.portainer_endpoint_registries.host.dockerhub_rate_limit == 0
    error_message = "the rate limit must stay zero when no registry is named."
  }
}
