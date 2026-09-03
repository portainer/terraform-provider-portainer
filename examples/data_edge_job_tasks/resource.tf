data "portainer_edge_job_tasks" "nightly" {
  edge_job_id = 1
}

output "tasks_with_logs" {
  value = [
    for t in data.portainer_edge_job_tasks.nightly.tasks : t.endpoint_name
    if t.logs_collected
  ]
}
