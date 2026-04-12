variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  # default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  # default     = "your-api-key-from-portainer"
}

variable "rule_id" {
  description = "ID of the alerting rule to configure"
  type        = number
  default     = 1
}

variable "rule_enabled" {
  description = "Whether the alerting rule is enabled"
  type        = bool
  default     = true
}

variable "rule_threshold" {
  description = "Threshold value for the alerting rule"
  type        = number
  default     = 90.0
}

variable "rule_duration" {
  description = "Duration in minutes before the alert fires"
  type        = number
  default     = 5
}
