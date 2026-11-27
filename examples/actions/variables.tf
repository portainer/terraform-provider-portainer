variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "environment_id" {
  description = "Environment (endpoint) to snapshot; null snapshots every environment"
  type        = number
  default     = null
}

variable "release" {
  description = "Release identifier; changing it triggers the snapshot"
  type        = string
  default     = "1"
}
