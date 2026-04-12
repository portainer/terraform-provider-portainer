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

variable "alert_manager_url" {
  description = "URL of the Alertmanager instance"
  type        = string
  default     = "http://localhost:9093"
}

variable "silence_comment" {
  description = "Comment describing the silence reason"
  type        = string
  default     = "Maintenance window"
}

variable "silence_starts_at" {
  description = "Start time of the silence in RFC3339 format"
  type        = string
  default     = "2026-04-12T00:00:00Z"
}

variable "silence_ends_at" {
  description = "End time of the silence in RFC3339 format"
  type        = string
  default     = "2026-04-12T06:00:00Z"
}

variable "matcher_name" {
  description = "Name of the matcher label"
  type        = string
  default     = "alertname"
}

variable "matcher_value" {
  description = "Value to match against"
  type        = string
  default     = "HighCPU"
}

variable "matcher_is_regex" {
  description = "Whether the matcher value is a regex"
  type        = bool
  default     = false
}

variable "matcher_is_equal" {
  description = "Whether the matcher is an equality matcher"
  type        = bool
  default     = true
}
