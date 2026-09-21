resource "portainer_omni_node_reboot" "cp_1" {
  credential_id = 1
  cluster       = "edge-fleet"
  node          = "cp-1"

  triggers = {
    after_upgrade = "v1.8.0"
  }
}
