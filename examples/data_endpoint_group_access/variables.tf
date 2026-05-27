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
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "endpoint_group_access_endpoint_group_id" {
  description = "Identifier of the Portainer endpoint group whose access policy is looked up"
  type        = number
  default     = 1
}

variable "endpoint_group_access_team_id" {
  description = "Identifier of the Portainer team to resolve the access policy for (either team_id or user_id must be set)"
  type        = number
  default     = 1
}
