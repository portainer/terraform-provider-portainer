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
  type        = number
  description = "ID of the Portainer endpoint/environment"
  default     = 3
}

variable "portainer_skip_ssl_verify" {
  description = "Set to true to skip TLS certificate verification (useful for self-signed certs)"
  type        = bool
  default     = true
}

variable "remote" {
  type        = string
  description = "Remote reference of the plugin to install (e.g. rclone/docker-volume-rclone)"
  default     = "rclone/docker-volume-rclone"
}

variable "name" {
  type        = string
  description = "Local alias name under which the plugin will be registered (e.g. rclone)"
  default     = "rclone"
}

variable "settings" {
  type = list(object({
    name  = string
    value = list(string)
  }))

  description = <<EOT
List of plugin permission settings required for rclone/docker-volume-rclone plugin.
Each object must define:
  - name: setting type (e.g. "network", "mount", "device", "capabilities")
  - value: list of string values for the setting

Defaults correspond to:
  - network: ["host"]
  - mount: [/var/lib/docker-plugins/rclone/config, /var/lib/docker-plugins/rclone/cache]
  - device: [/dev/fuse]
  - capabilities: [CAP_SYS_ADMIN]
EOT

  default = [
    {
      name  = "network"
      value = ["host"]
    },
    {
      name  = "mount"
      value = ["/var/lib/docker-plugins/rclone/config"]
    },
    {
      name  = "mount"
      value = ["/var/lib/docker-plugins/rclone/cache"]
    },
    {
      name  = "device"
      value = ["/dev/fuse"]
    },
    {
      name  = "capabilities"
      value = ["CAP_SYS_ADMIN"]
    }
  ]
}

