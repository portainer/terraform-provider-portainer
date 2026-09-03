# Validate manifests against the live cluster before anything is applied.
data "portainer_kubernetes_manifest_dry_run" "app" {
  environment_id = var.environment_id
  namespace      = "default"

  manifests = [
    file("${path.module}/configmap.yaml"),
    file("${path.module}/service.yaml"),
  ]

  # Turn an invalid manifest into a plan-time failure instead of a reported result.
  fail_on_invalid = true
}

output "dry_run_passed" {
  value = data.portainer_kubernetes_manifest_dry_run.app.passed
}

output "dry_run_failures" {
  value = [
    for r in data.portainer_kubernetes_manifest_dry_run.app.results : "${r.kind}/${r.name}: ${r.message}"
    if r.status != "pass"
  ]
}
