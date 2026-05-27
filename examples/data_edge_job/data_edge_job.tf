data "portainer_edge_job" "example" {
  name = var.edge_job_name
}

output "edge_job_id" {
  value = data.portainer_edge_job.example.id
}

output "edge_job_cron_expression" {
  value = data.portainer_edge_job.example.cron_expression
}
