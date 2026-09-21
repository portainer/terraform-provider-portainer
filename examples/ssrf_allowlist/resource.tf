resource "portainer_ssrf_allowlist" "main" {
  mode = "enforce"

  entries = [
    "https://hooks.example.com",
    "https://registry.example.com",
  ]
}
