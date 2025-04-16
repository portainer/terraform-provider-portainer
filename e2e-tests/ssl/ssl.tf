resource "portainer_ssl" "cert_update" {
  cert         = file(var.ssl_cert_path)
  key          = file(var.ssl_key_path)
  http_enabled = var.ssl_http_enabled
}
