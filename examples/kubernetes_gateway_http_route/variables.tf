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
  description = "Environment (Endpoint) identifier of the Kubernetes environment"
  type        = number
}

variable "gateway_name" {
  description = "Gateway the route attaches to"
  type        = string
  default     = "edge"
}

variable "service_name" {
  description = "Service the route forwards to"
  type        = string
  default     = "app"
}
