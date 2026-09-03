# Data Source Documentation: `portainer_kubernetes_manifest_dry_run`

# portainer_kubernetes_manifest_dry_run
The `portainer_kubernetes_manifest_dry_run` data source validates Kubernetes manifests against a live cluster through Portainer's generic dry-run API, without creating or changing anything. It is the Terraform equivalent of `kubectl apply --dry-run=server`, and catches schema errors, admission-webhook rejections and missing prerequisites while the plan is still being made.

Requires Portainer **2.45.0** or newer.

## Example Usage

### Report the outcome

```hcl
data "portainer_kubernetes_manifest_dry_run" "app" {
  environment_id = 1
  namespace      = "default"

  manifests = [
    file("configmap.yaml"),
    file("service.yaml"),
  ]
}

output "dry_run_failures" {
  value = [
    for r in data.portainer_kubernetes_manifest_dry_run.app.results : "${r.kind}/${r.name}: ${r.message}"
    if r.status != "pass"
  ]
}
```

### Fail the plan on an invalid manifest

```hcl
data "portainer_kubernetes_manifest_dry_run" "gate" {
  environment_id  = 1
  manifests       = [file("deployment.yaml")]
  fail_on_invalid = true
}
```

## Arguments Reference

| Name              | Type   | Required | Default | Description                                                                                                                                          |
|-------------------|--------|----------|---------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `environment_id`  | number | Yes      | –       | Environment (endpoint) identifier of the Kubernetes environment to validate against.                                                                  |
| `manifests`       | list   | Yes      | –       | Manifests to validate, as YAML or JSON strings. Each entry may hold several documents separated by `---`. At least one non-empty manifest is required. |
| `namespace`       | string | No       | –       | Namespace applied to documents that do not declare one. Leave unset to validate the manifests exactly as written.                                      |
| `fail_on_invalid` | bool   | No       | `false` | Whether a failing document makes the data source itself fail, instead of only reporting the outcome in `results` and `passed`.                        |

## Attributes Reference

| Name           | Type   | Description                                                                                                                       |
|----------------|--------|-----------------------------------------------------------------------------------------------------------------------------------|
| `passed`       | bool   | Whether every submitted document passed validation.                                                                                |
| `failed_count` | number | Number of documents that failed validation.                                                                                        |
| `results`      | list   | Per-document outcome, in submission order. Each entry has `kind`, `name`, `namespace`, `document_index`, `status` (`pass`/`fail`) and `message`. |

`kind` and `name` are empty for a document rejected before it could be identified (malformed YAML, for instance); `document_index` — the zero-based position among all non-empty documents submitted — is what identifies it in that case.
