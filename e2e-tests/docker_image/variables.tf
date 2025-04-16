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
  description = "ID of the Portainer environment where the image should be pulled"
  type        = number
  default     = "3"
}

variable "image" {
  description = "Docker image including tag (e.g., nginx:alpine)"
  type        = string
  default     = "nginx:latest"
}
