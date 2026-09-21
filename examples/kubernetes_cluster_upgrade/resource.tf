resource "portainer_kubernetes_cluster_upgrade" "prod" {
  endpoint_id = 1

  triggers = {
    # Change this to run another upgrade.
    round = "1"
  }
}
