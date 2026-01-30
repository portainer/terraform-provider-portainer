variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "portainer_skip_ssl_verify" {
  type        = bool
  description = "Skip SSL verification"
  default     = true
}

variable "team_name" {
  type        = string
  description = "Name of the team"
  default     = "e2e-access-team"
}

variable "endpoint_group_name" {
  type        = string
  description = "Name of the endpoint group"
  default     = "e2e-access-group"
}

variable "endpoint_group_description" {
  type        = string
  description = "Description of the endpoint group"
  default     = "E2E Test Group for Access Control"
}
