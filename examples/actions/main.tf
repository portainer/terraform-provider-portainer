terraform {
  # Actions need Terraform >= 1.14; OpenTofu does not support them.
  required_version = ">= 1.14"
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

# Snapshot one environment, or every environment when environment_id is null.
action "portainer_endpoint_snapshot" "refresh" {
  config {
    endpoint_id = var.environment_id
  }
}

# Stands in for whatever changes the environments (a stack, an image tag...):
# every change of var.release snapshots them afterwards. Nothing is kept in
# state for the snapshot itself.
resource "terraform_data" "release" {
  input = var.release

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.portainer_endpoint_snapshot.refresh]
    }
  }
}
