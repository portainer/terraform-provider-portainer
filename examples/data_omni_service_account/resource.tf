data "portainer_omni_service_account" "check" {
  endpoint            = "https://omni.example.com"
  service_account_key = var.secret
  fail_on_error       = false
}

output "omni_credential_valid" {
  value = data.portainer_omni_service_account.check.valid
}
