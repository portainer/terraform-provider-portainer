resource "portainer_tls" "upload_cert" {
  certificate = var.certificate
  folder      = var.folder
  file_path   = var.file_path
}
