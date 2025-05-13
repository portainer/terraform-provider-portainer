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

variable "environment_id" {
  type        = number
  description = "ID of the Portainer environment"
}

variable "device_id" {
  type        = number
  description = "ID of the AMT-managed device"
}

variable "device_action" {
  type        = string
  description = "Action to perform (e.g. poweron, poweroff, reset)"
  default     = "poweron"
}
