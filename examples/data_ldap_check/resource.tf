# fail_on_error defaults to true here: applying a broken LDAP configuration in
# the same plan would lock users out.
data "portainer_ldap_check" "corp" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=reader,dc=example,dc=com"
  password  = var.secret
  start_tls = true
}

output "ldap_reachable" {
  value = data.portainer_ldap_check.corp.success
}
