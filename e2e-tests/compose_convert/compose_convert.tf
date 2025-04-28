resource "portainer_compose_convert" "docker-compose-yml" {
  compose_content = file("${path.module}/docker-compose.yml")
}

resource "local_file" "k8s_manifests" {
  for_each = portainer_compose_convert.docker-compose-yml.manifests

  filename = "${path.module}/output/${each.key}"
  content  = each.value
}
