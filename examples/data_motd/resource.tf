data "portainer_motd" "this" {}

output "portainer_announcement" {
  value = data.portainer_motd.this.message
}
