# Run with `terraform query` (Terraform 1.14+). OpenTofu does not support
# list resources.

# Every tag, with the full resource state.
list "portainer_tag" "all" {
  provider         = portainer
  include_resource = true
}

# The environments of one environment group.
list "portainer_environment" "group" {
  provider = portainer

  config {
    group_id = var.environment_group_id
  }
}

# Administrators only.
list "portainer_user" "admins" {
  provider = portainer

  config {
    role = 1
  }
}

# Every other list resource of the provider, unfiltered.
list "portainer_team" "all" {
  provider = portainer
}

list "portainer_endpoint_group" "all" {
  provider = portainer
}

list "portainer_edge_group" "all" {
  provider = portainer
}

list "portainer_registry" "all" {
  provider = portainer
}

list "portainer_custom_template" "all" {
  provider = portainer
}

# On Business Edition set endpoint_id and resource_id.
list "portainer_webhook" "all" {
  provider = portainer
}

# Docker objects of one environment (endpoint_id is required).
list "portainer_docker_network" "all" {
  provider = portainer

  config {
    endpoint_id = var.docker_endpoint_id
  }
}

list "portainer_docker_volume" "all" {
  provider         = portainer
  include_resource = true

  config {
    endpoint_id = var.docker_endpoint_id
  }
}

# Swarm only: secrets, configs and nodes. portainer_docker_plugin works the
# same way on a standalone Docker host.
list "portainer_docker_secret" "all" {
  provider = portainer

  config {
    endpoint_id = var.docker_endpoint_id
  }
}

# Docker standalone, Swarm and Kubernetes stacks; method is inferred
# ("repository" for Git stacks, otherwise "string").
list "portainer_stack" "docker" {
  provider = portainer

  config {
    endpoint_id = var.docker_endpoint_id
  }
}

# Kubernetes objects of one environment (endpoint_id is required). With
# include_resource, manifest holds the live object, so
# `terraform query -generate-config-out` writes a usable configuration.
list "portainer_kubernetes_application" "default" {
  provider         = portainer
  include_resource = true

  config {
    endpoint_id = var.kubernetes_endpoint_id
    namespace   = "default"
  }
}

list "portainer_kubernetes_configmaps" "all" {
  provider = portainer

  config {
    endpoint_id = var.kubernetes_endpoint_id
  }
}

list "portainer_kubernetes_clusterrole" "all" {
  provider = portainer

  config {
    endpoint_id = var.kubernetes_endpoint_id
  }
}

# Edge stacks and jobs; Edge configurations and GitOps sources and
# workflows need Business Edition.
list "portainer_edge_stack" "all" {
  provider = portainer
}

list "portainer_edge_job" "all" {
  provider = portainer
}
