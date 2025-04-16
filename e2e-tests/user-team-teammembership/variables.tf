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

variable "portainer_user_username" {
  description = "Portainer username used for resource provisioning"
  type        = string
  default     = "your-user"
}

variable "portainer_user_password" {
  description = "Portainer password used for resource provisioning"
  type        = string
  sensitive   = true
  default     = "your-user-password"
}

variable "portainer_user_role" {
  description = "Role to assign to the Portainer user"
  type        = number
  default     = 2 # 1 = admin, 2 = standard user
}

variable "portainer_team_name" {
  description = "Portainer Team Name"
  type        = string
  default     = "your-team"
}

variable "team_membership_role" {
  description = "Membership role in the team: 1 = leader, 2 = member"
  type        = number
  default     = 2
}
