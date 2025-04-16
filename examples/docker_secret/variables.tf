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
  description = "ID endpointu v Portaineru"
  type        = number
  default     = 1
}

variable "secret_name" {
  description = "Název Docker secretu"
  type        = string
  default     = "app-key.crt"
}

variable "secret_data" {
  description = "Base64-encoded data pro secret"
  type        = string
  sensitive   = true
  default     = base64encode("THIS IS NOT A REAL CERTIFICATE\n")
}

variable "secret_labels" {
  description = "Map Docker secret labelů"
  type        = map(string)
  default = {
    "com.example.some-label" = "some-value"
  }
}

variable "secret_templating" {
  description = "Templating konfigurace"
  type        = map(string)
  default = {
    name    = "some-driver"
    OptionA = "value for driver-specific option A"
  }
}
