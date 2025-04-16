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

variable "environment_id" {
  type        = number
  description = "Portainer environment (Kubernetes endpoint) ID"
  default     = 4
}

variable "namespace" {
  type        = string
  description = "Kubernetes namespace to toggle system state for"
  default     = "default"
}

variable "system" {
  type        = bool
  description = "Whether the namespace should be marked as a system namespace"
  default     = true
}

