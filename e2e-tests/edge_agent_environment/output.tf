output "edge_key" {
  value = portainer_environment.edge_agent.edge_key
  # sensitive = true
}

output "edge_id" {
  value = portainer_environment.edge_agent.edge_id

  # The Edge Agent cannot register without an Edge ID, and Portainer leaves the
  # endpoint's EdgeID empty until an agent associates unless the EnforceEdgeID
  # setting is on. Fail the apply here rather than starting an agent with an
  # empty PORTAINER_EDGE_ID (issue #142).
  precondition {
    condition     = portainer_environment.edge_agent.edge_id != ""
    error_message = "edge_id must not be empty for an Edge Agent environment (issue #142)."
  }
}

output "environment_address" {
  value = portainer_environment.edge_agent.environment_address

  # Portainer stores an edge endpoint's URL as a bare host, dropping the scheme,
  # the port and the path, so "http://portainer:9000" comes back as "portainer"
  # and drifts on every refresh (issue #136). The provider reads the address
  # back from the edge key instead, which keeps it verbatim.
  #
  # The workflow re-evaluates this output after a refresh-only apply, so a
  # regression fails there rather than silently producing a perpetual diff. The
  # check is deliberately narrower than "the plan is empty": an unrelated
  # resource drifting must not make this look like an address regression.
  precondition {
    condition     = portainer_environment.edge_agent.environment_address == var.edge_agent_address
    error_message = "environment_address must survive a refresh unchanged (issue #136)."
  }
}
