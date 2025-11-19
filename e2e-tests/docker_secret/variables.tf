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
  description = "Base64ncoded data for secret"
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

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "portainer_team_name" {
  description = "Portainer Team Name"
  type        = string
  default     = "your-team-test"
}

variable "resource_control_type" {
  description = "Portainer ResourceControl type"
  type        = number
  default     = 5
}

variable "resource_control_administrators_only" {
  description = "Only administrators can access the resource"
  type        = bool
  default     = false
}

variable "resource_control_public" {
  description = "Whether the resource is public"
  type        = bool
  default     = false
}
