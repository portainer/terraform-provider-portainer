variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "endpoint_id" {
  description = "Environment (Endpoint) identifier"
  type        = number
}

variable "release_name" {
  description = "Name of the Helm release to rollback"
  type        = string
}

variable "revision" {
  description = "Revision number to rollback to"
  type        = number
}
