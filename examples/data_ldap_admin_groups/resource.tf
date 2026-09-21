data "portainer_ldap_admin_groups" "admins" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.secret

  admin_group_search {
    group_base_dn = "ou=groups,dc=example,dc=com"
    group_filter  = "(cn=*-admins)"
  }
}

output "portainer_admin_groups" {
  value = data.portainer_ldap_admin_groups.admins.group_names
}
