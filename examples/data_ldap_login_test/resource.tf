data "portainer_ldap_login_test" "alice" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.secret

  search {
    base_dn             = "ou=people,dc=example,dc=com"
    user_name_attribute = "uid"
  }

  username      = "alice"
  test_password = var.secret
}

output "ldap_login_valid" {
  value = data.portainer_ldap_login_test.alice.valid
}
