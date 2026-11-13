variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "rule_name" {
  description = "Name of the alerting rule to look up"
  type        = string
  default     = "Node NotReady"
}
