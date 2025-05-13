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

variable "environment_id" {
  type        = number
  default     = 1
  description = "Portainer environment ID (agent endpoint)"
}

variable "device_id" {
  type        = number
  default     = 42
  description = "ID of the AMT managed device"
}

variable "user_consent" {
  type        = string
  default     = "kvmOnly"
  description = "User consent policy (e.g. none, all, kvmOnly)"
}

variable "ider" {
  type        = bool
  default     = true
  description = "Enable IDER (IDE Redirection)"
}

variable "kvm" {
  type        = bool
  default     = true
  description = "Enable KVM (Keyboard/Video/Mouse)"
}

variable "sol" {
  type        = bool
  default     = true
  description = "Enable SOL (Serial Over LAN)"
}

variable "redirection" {
  type        = bool
  default     = true
  description = "Enable redirection"
}
