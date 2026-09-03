# The repository index.
data "portainer_helm_chart" "repo" {
  repo = "https://charts.bitnami.com/bitnami"
}

# A chart's default values, to diff against your own.
data "portainer_helm_chart" "nginx_values" {
  repo    = "https://charts.bitnami.com/bitnami"
  chart   = "nginx"
  command = "values"
}

output "nginx_defaults" {
  value = data.portainer_helm_chart.nginx_values.content
}
