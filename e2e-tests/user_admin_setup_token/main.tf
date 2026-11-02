terraform {
  required_providers {
    portainer = {
      source = "portainer/portainer"
    }
  }
}

# No credentials: the instance has no administrator yet, and the init call is
# public.
provider "portainer" {
  endpoint        = var.portainer_url
  skip_ssl_verify = true
}
