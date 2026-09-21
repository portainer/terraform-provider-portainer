# The template catalogue and a Helm repository index. Both make Portainer fetch
# from the internet, so a network hiccup shows up here as a failed apply rather
# than a provider bug - hence no preconditions on the contents.

data "portainer_app_templates" "catalogue" {}

data "portainer_helm_chart" "repo_index" {
  repo = "https://charts.bitnami.com/bitnami"
}

output "template_count" {
  value = length(data.portainer_app_templates.catalogue.templates)
}

output "catalogue_version" {
  value = data.portainer_app_templates.catalogue.version
}

output "helm_index_fetched" {
  value = length(data.portainer_helm_chart.repo_index.content) > 0
}
