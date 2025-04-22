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

variable "endpoint_id" {
  description = "ID of the Portainer endpoint"
  type        = number
  default     = 3
}

variable "secret_name" {
  description = "Name of Docker secret"
  type        = string
  default     = "app-key.crt"
}

variable "secret_data" {
  description = "Base64-encoded data for secret"
  type        = string
  sensitive   = true
  default     = "THIS IS NOT A REAL CERTIFICATE\n"
}

variable "secret_labels" {
  description = "Map Docker secret labels"
  type        = map(string)
  default = {
    "com.example.some-label" = "some-value"
  }
}

variable "secret_templating" {
  description = "Template configuration"
  type        = map(string)
  default = {
    name    = "some-driver"
    OptionA = "value for driver-specific option A"
  }
}
