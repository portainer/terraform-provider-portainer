resource "portainer_edge_job" "string_job" {
  name            = var.edge_job_name
  cron_expression = var.edge_job_cron
  edge_groups     = var.edge_job_edge_groups
  endpoints       = var.edge_job_endpoints
  recurring       = var.edge_job_recurring
  file_content    = var.edge_job_file_content
}
