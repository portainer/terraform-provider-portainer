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

variable "portainer_registry_name" {
  description = "Custom Registry"
  type        = string
  default     = "DockerHub"
}

variable "portainer_registry_type" {
  description = "Type registry"
  type        = number
  default     = 3
}

variable "portainer_registry_url" {
  description = "URL adresa registru"
  type        = string
  default     = "test-reegistry-docker.com"
}

variable "portainer_registry_authentication" {
  description = "Required use authentication"
  type        = bool
  default     = false
}
