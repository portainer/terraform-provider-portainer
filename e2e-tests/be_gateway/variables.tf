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

variable "k8s_environment_id" {
  description = "ID of the k3d environment e2e-tests/lib/be-k8s-setup.sh connects"
  type        = number
}

variable "port" {
  description = "Listener port; run.sh changes it to test an update"
  type        = number
  default     = 80
}
