resource "portainer_settings" "example" {
  authentication_method        = var.authentication_method
  enable_telemetry             = var.enable_telemetry
  logo_url                     = var.logo_url
  snapshot_interval            = var.snapshot_interval
  user_session_timeout         = var.user_session_timeout
  enable_edge_compute_features = var.enable_edge_compute_features
  enforce_edge_id              = var.enforce_edge_id

  internal_auth_settings {
    required_password_length = var.required_password_length
  }

  ldap_settings {
    anonymous_mode    = var.ldap_anonymous_mode
    auto_create_users = var.ldap_auto_create_users
    password          = var.ldap_password
    reader_dn         = var.ldap_reader_dn
    start_tls         = var.ldap_start_tls
    url               = var.ldap_url
  }
}
