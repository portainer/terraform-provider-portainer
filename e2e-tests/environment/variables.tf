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

variable "portainer_environment_name" {
  description = "Portainer environment name"
  type        = string
  default     = "Your test environment name"
}

variable "portainer_environment_address" {
  description = "Portainer environment address"
  type        = string
  default     = "tcp://host:9001"
}

variable "portainer_environment_type" {
  description = "Portainer environment type"
  type        = number
  default     = 2 # Environment type: `1` = Docker, `2` = Agent, `3` = Azure, `4` = Edge Agent, `5` = Kubernetes.
}
