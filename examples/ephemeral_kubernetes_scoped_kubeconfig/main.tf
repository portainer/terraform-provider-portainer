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

# Issued when Terraform opens it and revoked (with its service account and
# role bindings) when the run ends. Requires Portainer BE 2.45.2+.
ephemeral "portainer_kubernetes_scoped_kubeconfig" "reader" {
  environment_id = var.environment_id
  expiry_seconds = 3600
  namespaces     = [var.namespace]
  description    = "terraform run"
}

locals {
  kubeconfig = yamldecode(ephemeral.portainer_kubernetes_scoped_kubeconfig.reader.kubeconfig)
}

provider "kubernetes" {
  host     = local.kubeconfig.clusters[0].cluster.server
  token    = local.kubeconfig.users[0].user.token
  insecure = true
}

data "kubernetes_namespace_v1" "ns" {
  metadata {
    name = var.namespace
  }
}
