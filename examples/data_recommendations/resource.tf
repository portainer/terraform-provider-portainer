data "portainer_recommendations" "high" {
  severity = "high"
}

output "high_severity_recommendations" {
  value = [for r in data.portainer_recommendations.high.recommendations : r.title]
}

output "recommendation_total" {
  value = data.portainer_recommendations.high.total
}
