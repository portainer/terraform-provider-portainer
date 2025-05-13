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

variable "certificate" {
  type        = string
  default     = "cert" # nebo "ca", "key"
  description = "Type of TLS file to upload: 'cert', 'ca', or 'key'"
}

variable "folder" {
  type        = string
  default     = "my-endpoint-folder"
  description = "Destination folder in Portainer to store the TLS file"
}

variable "file_path" {
  type        = string
  default     = "my-cert.pem"
  description = "Path to the local TLS file to upload"
}

