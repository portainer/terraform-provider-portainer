variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "endpoint_id" {
  description = "ID of the Portainer endpointr"
  type        = number
  default     = 3
}

variable "config_name" {
  description = "Name Docker config"
  type        = string
  default     = "server.conf"
}

variable "config_data" {
  description = "Base64ncoded data for Docker config"
  type        = string
  sensitive   = true
  default     = "THIS IS NOT A REAL CERTIFICATE\n"
}

variable "config_labels" {
  description = "Map Docker config labels"
  type        = map(string)
  default = {
    property1 = "string"
    property2 = "string"
    foo       = "bar"
  }
}

variable "config_templating" {
  description = "Templating configuration"
  type        = map(string)
  default = {
    name    = "some-driver"
    OptionA = "value for driver-specific option A"
    OptionB = "value for driver-specific option B"
  }
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}
