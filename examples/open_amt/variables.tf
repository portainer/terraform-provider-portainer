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

variable "enabled" {
  description = "Enable or disable OpenAMT"
  type        = bool
  default     = true
}

variable "domain_name" {
  description = "Domain name for OpenAMT"
  type        = string
}

variable "mpsserver" {
  description = "URL of the MPS (Management Presence Server)"
  type        = string
}

variable "mpsuser" {
  description = "Username for MPS server"
  type        = string
}

variable "mpspassword" {
  description = "Password for MPS server"
  type        = string
  sensitive   = true
}

variable "cert_file_name" {
  description = "Name of the PFX certificate file"
  type        = string
}

variable "cert_file_password" {
  description = "Password for the PFX certificate"
  type        = string
  sensitive   = true
}

variable "cert_file_path" {
  description = "Path to the local PFX certificate file (base64 encoded via filebase64)"
  type        = string
}
