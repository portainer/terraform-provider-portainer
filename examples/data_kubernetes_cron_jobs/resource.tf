data "portainer_kubernetes_cron_jobs" "all" {
  endpoint_id = 1
}

output "kubernetes_suspended_cron_jobs" {
  value = [for j in data.portainer_kubernetes_cron_jobs.all.cron_jobs : j.name if j.suspend]
}
