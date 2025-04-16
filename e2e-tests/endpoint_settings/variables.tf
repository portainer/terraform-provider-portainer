variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
  default     = "http://localhost:9000"
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
  default     = "ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8="
}

variable "endpoint_id" {
  type        = number
  description = "ID of the Portainer endpoint"
  default     = 3
}

variable "allow_bind_mounts" {
  type        = bool
  default     = true
  description = "Allow bind mounts for regular users"
}

variable "allow_container_capabilities" {
  type        = bool
  default     = true
  description = "Allow container capabilities for regular users"
}

variable "allow_device_mapping" {
  type        = bool
  default     = true
  description = "Allow device mapping for regular users"
}

variable "allow_host_namespace" {
  type        = bool
  default     = true
  description = "Allow host namespace for regular users"
}

variable "allow_privileged_mode" {
  type        = bool
  default     = false
  description = "Allow privileged mode for regular users"
}

variable "allow_stack_management" {
  type        = bool
  default     = true
  description = "Allow stack management for regular users"
}

variable "allow_sysctl_setting" {
  type        = bool
  default     = true
  description = "Allow sysctl setting for regular users"
}

variable "allow_volume_browser" {
  type        = bool
  default     = true
  description = "Allow volume browser for regular users"
}

variable "enable_gpu_management" {
  type        = bool
  default     = false
  description = "Enable GPU management"
}

variable "enable_host_management" {
  type        = bool
  default     = true
  description = "Enable host management features"
}

variable "gpus" {
  type = list(object({
    name  = string
    value = string
  }))
  default = [
    {
      name  = "nvidia"
      value = "gpu0"
    }
  ]
  description = "List of GPU settings (name + value)"
}

