resource "portainer_ldap_settings" "ldap" {
  url       = var.ldap_url
  reader_dn = var.ldap_reader_dn
  password  = var.ldap_password

  search_settings {
    base_dn             = "dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }

  group_search_settings {
    group_base_dn   = "ou=groups,dc=example,dc=com"
    group_filter    = "(objectClass=groupOfNames)"
    group_attribute = "member"
  }

  tls_config {
    tls             = true
    tls_skip_verify = false
  }
}
