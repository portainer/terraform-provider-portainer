# An Edge Agent environment for a machine that installs KubeSolo on its first
# boot. Portainer turns it into a Kubernetes Edge Agent environment once the
# agent KubeSolo deploys checks in.
resource "portainer_environment" "kubesolo" {
  name                = var.environment_name
  environment_address = var.portainer_edge_url
  type                = 4
}

# cloud-init user data that installs KubeSolo and joins it to the environment
# above. Pass it to whatever creates the machine (the user_data of a cloud
# instance, the cloud-init drive of a VM). It carries the edge key, so it is
# sensitive.
output "kubesolo_user_data" {
  sensitive = true
  value     = <<-EOT
    #cloud-config
    runcmd:
      - - sh
        - -c
        - >-
          curl -sfL https://get.kubesolo.io |
          KUBESOLO_PORTAINER_EDGE_ID='${portainer_environment.kubesolo.edge_id}'
          KUBESOLO_PORTAINER_EDGE_KEY='${portainer_environment.kubesolo.edge_key}'
          sudo -E sh
  EOT
}
