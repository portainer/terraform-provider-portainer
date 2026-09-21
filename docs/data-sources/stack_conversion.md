# Data Source Documentation: `portainer_stack_conversion`

# portainer_stack_conversion

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Converts a Compose stack to Kubernetes manifests or a Helm chart and hands the result back for preview.

## Example Usage

```hcl
data "portainer_stack_conversion" "web" {
  stack_id      = portainer_stack.web.id
  target_format = "kubernetes"
  namespace     = "apps"
}

resource "local_file" "manifests" {
  for_each = data.portainer_stack_conversion.web.files

  filename = "${path.module}/converted/${each.key}"
  content  = each.value
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `stack_id` | number | ✅ yes | Compose stack to convert. |
| `target_format` | string | ✅ yes | `kubernetes` for manifests or `helm` for a Helm chart. |
| `namespace` | string | ❌ no | Namespace to generate the Kubernetes resources into. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `files` | map(string) | The converted files, keyed by file name. |

This is a data source rather than a resource because the endpoint changes nothing: it hands back files to look at, and deploying them is a separate decision.
