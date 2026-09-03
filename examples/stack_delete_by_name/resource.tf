# Clears a Kubernetes stack Terraform never created, before adopting the
# namespace. Use portainer_stack for stacks Terraform owns.
resource "portainer_stack_delete_by_name" "legacy" {
  name        = "legacy-app"
  endpoint_id = 4
  external    = true
}
