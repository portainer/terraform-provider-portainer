resource "portainer_open_amt" "enable" {
  enabled     = var.enabled
  domain_name = var.domain_name
  mpsserver   = var.mpsserver
  mpsuser     = var.mpsuser
  mpspassword = var.mpspassword

  cert_file_name     = var.cert_file_name
  cert_file_password = var.cert_file_password
  cert_file_content  = filebase64(var.cert_file_path)
}
