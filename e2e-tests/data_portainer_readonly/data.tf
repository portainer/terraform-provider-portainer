# Instance-level read-only data sources. No setup, no cluster, no network beyond
# Portainer itself, so this is the cheapest e2e coverage in the suite.

data "portainer_system" "this" {}

data "portainer_settings_public" "this" {}

data "portainer_motd" "this" {}

data "portainer_endpoints_summary" "fleet" {}

# No user_id: resolves whichever account the API key belongs to.
data "portainer_user_access" "me" {}

output "portainer_version" {
  value = data.portainer_system.this.server_version

  # A running instance always reports a version; an empty string means the
  # version endpoint was not read correctly.
  precondition {
    condition     = data.portainer_system.this.server_version != ""
    error_message = "portainer_system must report a server version."
  }
}

output "admin_initialized" {
  value = data.portainer_system.this.admin_initialized

  # The e2e suite runs against an initialised instance.
  precondition {
    condition     = data.portainer_system.this.admin_initialized
    error_message = "admin_initialized must be true for an initialised instance."
  }
}

output "environment_total" {
  value = data.portainer_endpoints_summary.fleet.total

  precondition {
    condition     = data.portainer_endpoints_summary.fleet.total > 0
    error_message = "the summary must count the environments the suite created."
  }
}

output "current_user" {
  value = data.portainer_user_access.me.username

  precondition {
    condition     = data.portainer_user_access.me.username != ""
    error_message = "portainer_user_access must resolve the calling user."
  }
}

output "edge_compute_enabled" {
  value = data.portainer_settings_public.this.enable_edge_compute_features
}

output "motd_title" {
  value = data.portainer_motd.this.title
}
