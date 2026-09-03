# Data Source Documentation: `portainer_helm_chart`

# portainer_helm_chart
Reads from a Helm repository through Portainer: the repository index, or a chart's default values, readme or metadata.

## Example Usage

```hcl
# The repository index.
data "portainer_helm_chart" "repo" {
  repo = "https://charts.bitnami.com/bitnami"
}

# A chart's default values, to diff against your own.
data "portainer_helm_chart" "nginx_values" {
  repo    = "https://charts.bitnami.com/bitnami"
  chart   = "nginx"
  command = "values"
}

output "nginx_defaults" {
  value = data.portainer_helm_chart.nginx_values.content
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `repo` | string | ✅ yes | URL of the Helm repository. |
| `chart` | string | ❌ no | Chart to inspect. Leave unset to fetch the repository index. |
| `command` | string | ❌ no | What to fetch for the chart: `values` (the default), `readme` or `chart`. Ignored when `chart` is unset. |
| `version` | string | ❌ no | Chart version to inspect. Leave unset for the newest. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `content` | string | The requested document, as returned by Portainer. |

Both endpoints answer with a document — YAML or Markdown — rather than JSON, so `content` is the raw text.
