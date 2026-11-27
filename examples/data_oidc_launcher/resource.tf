data "portainer_oidc_launcher" "this" {}

output "launcher_links" {
  value = { for e in data.portainer_oidc_launcher.this.entries : e.name => e.homepage_url }
}
