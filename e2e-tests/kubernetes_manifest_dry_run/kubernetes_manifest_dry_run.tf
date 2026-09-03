# Server-side validation only: this creates nothing in the cluster.
data "portainer_kubernetes_manifest_dry_run" "valid" {
  environment_id = var.endpoint_id
  namespace      = var.namespace
  manifests      = [file("${path.module}/configmap.yaml")]

  # A valid manifest must pass, so let the data source fail the apply itself.
  fail_on_invalid = true
}

# The counterpart: an invalid manifest must be reported as failing rather than
# silently accepted.
data "portainer_kubernetes_manifest_dry_run" "invalid" {
  environment_id = var.endpoint_id
  namespace      = var.namespace
  manifests      = [file("${path.module}/invalid.yaml")]
}

output "dry_run_valid_passed" {
  value = data.portainer_kubernetes_manifest_dry_run.valid.passed

  precondition {
    condition     = data.portainer_kubernetes_manifest_dry_run.valid.passed
    error_message = "A valid manifest must pass the server-side dry run."
  }
}

output "dry_run_invalid_failed" {
  value = data.portainer_kubernetes_manifest_dry_run.invalid.failed_count

  precondition {
    condition     = data.portainer_kubernetes_manifest_dry_run.invalid.failed_count > 0
    error_message = "An invalid manifest must be reported as failing the dry run."
  }
}
