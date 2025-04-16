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

variable "config_name" {
  description = "Název Docker configu"
  type        = string
  default     = "server.conf"
}

variable "config_data" {
  description = "Base64-encoded obsah Docker configu"
  type        = string
  sensitive   = true
  default     = base64encode("THIS IS NOT A REAL CERTIFICATE\n")
}

variable "config_labels" {
  description = "Map Docker config labelů"
  type        = map(string)
  default = {
    property1 = "string"
    property2 = "string"
    foo       = "bar"
  }
}

variable "config_templating" {
  description = "Templating konfigurace"
  type        = map(string)
  default = {
    name    = "some-driver"
    OptionA = "value for driver-specific option A"
    OptionB = "value for driver-specific option B"
  }
}
