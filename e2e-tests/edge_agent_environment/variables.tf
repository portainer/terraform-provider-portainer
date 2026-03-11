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

variable "edge_agent_name" {
  description = "Name of the edge agent environment"
  type        = string
  default     = "test-edge-agent"
}

variable "edge_agent_address" {
  description = "Address of the edge agent environment"
  type        = string
  default     = "tcp://edge"
}
