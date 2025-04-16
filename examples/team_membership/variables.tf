variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "user_username" {
  description = "Username for the Portainer user"
  type        = string
  default     = "testuser"
}

variable "user_password" {
  description = "Password for the Portainer user"
  type        = string
  sensitive   = true
  default     = "StrongPassword123!"
}

variable "user_role" {
  description = "User role: 1 = admin, 2 = standard"
  type        = number
  default     = 2
}

variable "user_ldap" {
  description = "Whether the user is an LDAP user"
  type        = bool
  default     = false
}

variable "team_name" {
  description = "Name of the Portainer team"
  type        = string
  default     = "test-team"
}

variable "team_membership_role" {
  description = "Membership role in the team: 1 = leader, 2 = member"
  type        = number
  default     = 2
}
