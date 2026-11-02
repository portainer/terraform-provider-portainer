variable "portainer_url" {
  description = "URL of the fresh Portainer instance run.sh starts for this test"
  type        = string
  default     = "https://localhost:19443"
}

variable "setup_token" {
  description = "Setup token printed in the fresh instance's log. Empty skips the header."
  type        = string
  sensitive   = true
  default     = ""
}
