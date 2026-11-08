variable "portainer_url" {
  description = "URL of the Portainer Business Edition instance e2e-tests/lib/be-setup.sh starts"
  type        = string
  default     = "https://localhost:9443"
}

variable "portainer_api_key" {
  description = "API key be-setup.sh creates"
  type        = string
  sensitive   = true
  default     = ""
}

variable "portainer_skip_ssl_verify" {
  description = "Skip TLS verification of the self-signed certificate"
  type        = bool
  default     = true
}

variable "endpoint_id" {
  description = "ID of the local Docker environment be-setup.sh creates"
  type        = number
  default     = 1
}

variable "name" {
  description = "Name of the cloud credentials (changed by the test to exercise update)"
  type        = string
  default     = "e2e-aws"
}
