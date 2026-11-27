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

# A new key pair on every plan and apply; the private key is never stored.
# Use it only where the key is needed during the run itself.
ephemeral "portainer_sshkeygen" "deploy" {}
