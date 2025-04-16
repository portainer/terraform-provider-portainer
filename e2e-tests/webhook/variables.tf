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

variable "endpoint_id" {
  description = "Portainer environment/endpoint ID"
  type        = number
  default     = 1
}

variable "resource_id" {
  description = "ID of the resource (e.g., stack ID or registry ID)"
  type        = string
  default     = "3"
}

variable "webhook_type" {
  description = "Type of the webhook"
  type        = number
  default     = 1
}
