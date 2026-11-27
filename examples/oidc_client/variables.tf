variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "portainer_public_url" {
  description = "URL browsers and add-ons reach Portainer at; the OIDC issuer is this URL followed by /api/oidc"
  type        = string
  default     = "https://portainer.example.com"
}

variable "grafana_url" {
  description = "URL of the Grafana instance that signs users in through Portainer"
  type        = string
  default     = "https://grafana.example.com"
}
