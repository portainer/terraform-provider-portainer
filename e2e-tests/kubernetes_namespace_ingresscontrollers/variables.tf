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
  description = "The ID of the Kubernetes environment (endpoint) in Portainer."
  default     = 4
}

variable "namespace" {
  type        = string
  description = "The name of the Kubernetes namespace where the ingress controllers should be applied."
  default     = "default"
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
  default = {
    name         = "nginx"
    class_name   = "nginx"
    type         = "ingress"
    availability = true
    used         = true
    new          = false
  }
}
