data "portainer_ldap_groups" "teams" {
  url       = "ldap.example.com:389"
  reader_dn = "cn=readonly,dc=example,dc=com"
  password  = var.secret

  group_search {
    group_base_dn   = "ou=groups,dc=example,dc=com"
    group_filter    = "(objectClass=groupOfNames)"
    group_attribute = "member"
  }
}

output "ldap_group_names" {
  value = [for g in data.portainer_ldap_groups.teams.entries : g.name]
}
