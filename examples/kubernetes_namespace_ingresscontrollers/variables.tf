variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "environment_id" {
  type        = number
  description = "The ID of the Kubernetes environment (endpoint) in Portainer."
}

variable "namespace" {
  type        = string
  description = "The name of the Kubernetes namespace where the ingress controllers should be applied."
}

variable "ingress_controller" {
  type = object({
    name         = string
    class_name   = string
    type         = string
    availability = bool
    used         = bool
    new          = bool
  })
  description = "Configuration for the Kubernetes ingress controller."
}
