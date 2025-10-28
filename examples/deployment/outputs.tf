# --- Docker image pull info ---
output "nginx_image" {
  description = "Pulled Nginx image information"
  value       = portainer_docker_image.pull.image
}

# --- Deploy output ---
output "deploy_output" {
  description = "Result log from deploy step"
  value       = portainer_deploy.deploy.output
}

# --- Exec output ---
output "exec_output" {
  description = "Output from container exec command"
  value       = portainer_container_exec.exec.output
}

# --- Check output ---
output "check_output" {
  description = "Verification check log"
  value       = portainer_check.check.output
}
