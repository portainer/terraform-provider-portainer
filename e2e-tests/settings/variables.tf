variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "authentication_method" {
  type        = number
  description = "Authentication method"
  default     = 1
}

variable "enable_telemetry" {
  type        = bool
  description = "Enable Portainer telemetry"
  default     = false
}

variable "logo_url" {
  type        = string
  description = "Custom logo URL"
  default     = "https://www.portainer.io/hubfs/portainer-logo-black.svg"
}

variable "snapshot_interval" {
  type        = string
  description = "Interval for snapshots (e.g., 15m)"
  default     = "15m"
}

variable "user_session_timeout" {
  type        = string
  description = "Session timeout duration (e.g., 8h)"
  default     = "8h"
}

variable "required_password_length" {
  type        = number
  description = "Minimum password length for internal auth"
  default     = 18
}

variable "ldap_anonymous_mode" {
  type        = bool
  description = "Enable anonymous LDAP mode"
  default     = true
}

variable "ldap_auto_create_users" {
  type        = bool
  description = "Auto-create users from LDAP"
  default     = true
}

variable "ldap_password" {
  type        = string
  description = "LDAP bind password"
  default     = "readonly"
  sensitive   = true
}

variable "ldap_reader_dn" {
  type        = string
  description = "LDAP Reader DN"
  default     = "cn=readonly-account,dc=example,dc=com"
}

variable "ldap_start_tls" {
  type        = bool
  description = "Enable StartTLS for LDAP"
  default     = true
}

variable "ldap_url" {
  type        = string
  description = "LDAP server URL"
  default     = "ldap.example.com:389"
}
