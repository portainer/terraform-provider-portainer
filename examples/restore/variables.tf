variable "portainer_url" {
  description = "Default Portainer URL"
  type        = string
}

variable "portainer_api_key" {
  description = "Default Portainer Admin API Key"
  type        = string
  sensitive   = true
}

variable "secret" {
  description = "Password or token used by this example"
  type        = string
  sensitive   = true
  default     = ""
}

variable "backup_archive_base64" {
  description = "Base64-encoded Portainer backup archive. Read it with filebase64(\"path/to/portainer-backup.tar.gz\") from the calling configuration; it is a variable here so the example validates without an archive committed to the repository."
  type        = string
  sensitive   = true
  default     = ""
}
