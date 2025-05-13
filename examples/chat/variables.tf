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

variable "chat_context" {
  type        = string
  default     = "environment_aware"
  description = "The context for the chat query (e.g., 'environment_aware')"
}

variable "chat_environment_id" {
  type        = number
  description = "ID of the Portainer environment where the chat applies"
}

variable "chat_message" {
  type        = string
  description = "The message or query to send to the Portainer chat endpoint"
}

variable "chat_model" {
  type        = string
  default     = "gpt-3.5-turbo"
  description = "OpenAI model to use (e.g., 'gpt-3.5-turbo', 'gpt-4')"
}
