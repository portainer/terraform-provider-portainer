data "portainer_policy_observability_test" "oneuptime" {
  one_uptime_url = "https://oneuptime.example.com"
  api_key        = var.secret
  fail_on_error  = false
}

output "oneuptime_reachable" {
  value = data.portainer_policy_observability_test.oneuptime.success
}
