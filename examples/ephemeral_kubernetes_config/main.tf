terraform {
  # Ephemeral resources need Terraform >= 1.10 (or OpenTofu >= 1.11).
  required_version = ">= 1.10"
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
    kubernetes = {
      source = "hashicorp/kubernetes"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}

# Generated on every plan and apply, never stored.
ephemeral "portainer_kubernetes_config" "env" {
  environment_ids = [var.environment_id]
}

locals {
  kubeconfig = yamldecode(ephemeral.portainer_kubernetes_config.env.kubeconfig)
}

provider "kubernetes" {
  host     = local.kubeconfig.clusters[0].cluster.server
  token    = local.kubeconfig.users[0].user.token
  insecure = true
}

data "kubernetes_namespace_v1" "default" {
  metadata {
    name = "default"
  }
}
