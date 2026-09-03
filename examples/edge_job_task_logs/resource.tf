data "portainer_edge_job_tasks" "nightly" {
  edge_job_id = 1
}

# Collection is asynchronous: the agent uploads on its next check-in.
resource "portainer_edge_job_task_logs" "collect" {
  edge_job_id = 1
  task_id     = data.portainer_edge_job_tasks.nightly.tasks[0].id
}

data "portainer_edge_job_task_logs" "nightly" {
  edge_job_id = 1
  task_id     = data.portainer_edge_job_tasks.nightly.tasks[0].id
}

output "nightly_output" {
  value = data.portainer_edge_job_task_logs.nightly.logs
}
