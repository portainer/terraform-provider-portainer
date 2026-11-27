terraform {
  # Ephemeral resources need Terraform >= 1.10 (or OpenTofu >= 1.11).
  required_version = ">= 1.10"
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

provider "portainer" {
  endpoint = var.portainer_url
  api_key  = var.portainer_api_key
}

# A general Edge key, generated on every plan and apply and never stored.
# Hand it to whatever installs the Edge Agent, e.g. a provisioner or a
# write-only argument of another provider.
ephemeral "portainer_endpoints_edge_generate_key" "edge" {}
