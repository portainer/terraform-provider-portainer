data "portainer_user_namespaces" "alice" {
  user_id = 1
}

output "user_writable_namespaces" {
  value = [
    for n in data.portainer_user_namespaces.alice.namespaces : "${n.endpoint_id}/${n.namespace}"
    if contains(n.authorizations, "K8sAccessNamespaceWrite")
  ]
}
