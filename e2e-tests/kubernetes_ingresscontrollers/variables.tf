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
  description = "ID of the Kubernetes environment (endpoint)."
  default     = 4
}

variable "controllers" {
  description = "List of ingress controller configurations."
  type = list(object({
    name         = string
    class_name   = string
    type         = string
    availability = bool
    used         = bool
    new          = bool
  }))
  default = [
    {
      name         = "nginx"
      class_name   = "nginx"
      type         = "ingress"
      availability = true
      used         = true
      new          = false
    }
  ]
}
