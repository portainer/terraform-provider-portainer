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

variable "portainer_endpoint_group_name" {
  description = "Name of the Portainer endpoint group"
  type        = string
  default     = "your-group"
}

variable "portainer_endpoint_group_description" {
  description = "Description of the group"
  type        = string
  default     = "Description for your group"
}
