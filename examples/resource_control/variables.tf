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

variable "resource_id" {
  description = "ID of the Docker/Kubernetes resource to control"
  type        = string
}

variable "resource_type" {
  description = "Type of the resource (e.g., 1 = container, 2 = volume, etc.)"
  type        = number
}

variable "administrators_only" {
  description = "Restrict access to administrators only"
  type        = bool
  default     = false
}

variable "public" {
  description = "Whether the resource should be public"
  type        = bool
  default     = false
}

variable "sub_resource_ids" {
  description = "List of sub-resource IDs (if any)"
  type        = list(string)
  default     = []
}

variable "teams" {
  description = "List of team IDs allowed to access the resource"
  type        = list(number)
  default     = []
}

variable "users" {
  description = "List of user IDs allowed to access the resource"
  type        = list(number)
  default     = []
}
