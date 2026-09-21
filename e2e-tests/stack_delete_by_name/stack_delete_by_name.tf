# Removing a stack that does not exist is the state this resource asks for, so
# Portainer's 404 is treated as success. That is the safe way to exercise the
# request path in CI without deleting anything the suite depends on.

resource "portainer_stack_delete_by_name" "absent" {
  name        = "e2e-stack-that-does-not-exist"
  endpoint_id = var.endpoint_id
  external    = false
}

output "delete_by_name_id" {
  value = portainer_stack_delete_by_name.absent.id
}
