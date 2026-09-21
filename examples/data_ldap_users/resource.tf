data "portainer_ldap_users" "people" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.secret

  search {
    base_dn             = "ou=people,dc=example,dc=com"
    filter              = "(objectClass=person)"
    user_name_attribute = "uid"
  }
}

output "ldap_user_names" {
  value = [for u in data.portainer_ldap_users.people.entries : u.name]
}
