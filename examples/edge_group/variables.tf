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

variable "edge_group_name" {
  description = "Name of the edge group"
  type        = string
  # default     = "dynamic-group"
}

variable "edge_group_dynamic" {
  description = "Whether the edge group is dynamic"
  type        = bool
  # default     = true
}

variable "edge_group_partial_match" {
  description = "Whether to use partial match when dynamic = true"
  type        = bool
  # default     = true
}

variable "edge_group_tag_ids" {
  description = "List of tag IDs used for dynamic matching"
  type        = list(number)
  # default     = [1] # Replace with actual tag IDs
}
