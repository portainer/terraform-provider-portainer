# The two pre-flight checks. CI has neither a registry nor an LDAP server, so
# both are expected to report failure - which is exactly the branch worth
# testing: the request must be made and the outcome reported, not raised.

data "portainer_registry_connection" "unreachable" {
  url           = "registry.invalid.example"
  type          = 3
  fail_on_error = false
}

data "portainer_ldap_check" "unreachable" {
  url           = "ldap.invalid.example:389"
  fail_on_error = false
}

output "registry_reported_outcome" {
  value = data.portainer_registry_connection.unreachable.success

  precondition {
    condition     = !data.portainer_registry_connection.unreachable.success
    error_message = "an unreachable registry must report success = false, not fail the plan."
  }
}

output "ldap_reported_outcome" {
  value = data.portainer_ldap_check.unreachable.success

  precondition {
    condition     = !data.portainer_ldap_check.unreachable.success
    error_message = "an unreachable directory must report success = false when fail_on_error is off."
  }
}
