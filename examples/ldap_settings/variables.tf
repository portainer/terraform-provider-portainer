variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "ldap_url" {
  description = "LDAP server URL"
  type        = string
}

variable "ldap_reader_dn" {
  description = "DN of the LDAP read-only account"
  type        = string
}

variable "ldap_password" {
  description = "Password for the LDAP read-only account"
  type        = string
  sensitive   = true
}
