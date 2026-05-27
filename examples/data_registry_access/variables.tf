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

variable "registry_id" {
  description = "Identifier of the Portainer registry whose access policy is queried"
  type        = number
  default     = 1
}

variable "endpoint_id" {
  description = "Identifier of the Portainer endpoint to which the registry access policy applies"
  type        = number
  default     = 1
}

variable "team_id" {
  description = "Identifier of the team whose access policy should be returned (mutually exclusive with user_id)"
  type        = number
  default     = 1
}
