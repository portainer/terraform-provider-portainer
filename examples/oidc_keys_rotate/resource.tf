resource "portainer_oidc_keys_rotate" "this" {
  triggers = {
    schedule = var.rotation_id
  }
}
