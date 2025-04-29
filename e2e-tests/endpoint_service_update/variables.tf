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

variable "endpoint_id" {
  description = "ID of the Portainer endpoint"
  type        = number
  default     = 3
}

variable "service_name" {
  description = "Name of the Docker service to force update"
  type        = string
  default     = "your-swarm-string_web"
}

variable "pull_image" {
  description = "Whether to pull the latest image before updating the service"
  type        = bool
  default     = true
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
