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
