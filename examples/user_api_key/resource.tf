resource "portainer_user_api_key" "terraform" {
  user_id     = 3
  description = "terraform"
  password    = var.user_password
}

# Portainer returns the raw key exactly once, at creation.
output "raw_api_key" {
  value     = portainer_user_api_key.terraform.raw_api_key
  sensitive = true
}

output "api_key_prefix" {
  value = portainer_user_api_key.terraform.prefix
}
