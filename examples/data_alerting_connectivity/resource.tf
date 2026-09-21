data "portainer_alerting_connectivity" "main" {
  url           = "http://alertmanager.example.com:9093"
  fail_on_error = false
}

output "alertmanager_reachable" {
  value = data.portainer_alerting_connectivity.main.reachable
}
