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

variable "repository_url" {
  description = "URL of the Git repository containing the Helm chart"
  type        = string
}
