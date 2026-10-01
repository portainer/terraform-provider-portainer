# Issue #147: a stack whose deployment failed must stay manageable from
# Terraform. Portainer keeps such a stack in status 4 (Error) until something
# deploys it again, so refusing to send the update that would fix it left the
# stack recoverable only by hand in the Portainer UI.
#
# Only the image changes between the three applies run.sh performs, which is
# enough: an unpullable tag fails the deployment without making the compose
# file itself invalid, so Portainer accepts the update and fails it afterwards
# — the asynchronous failure this issue is about.
resource "portainer_stack" "error_recovery" {
  name            = var.stack_name
  deployment_type = "standalone"
  method          = "string"
  endpoint_id     = var.stack_endpoint_id

  stack_file_content = <<-EOT
    version: "3"
    services:
      web:
        image: ${var.stack_image}
  EOT
}

output "stack_id" {
  description = "Stack identifier, used by run.sh to poll the deployment status."
  value       = portainer_stack.error_recovery.id
}
