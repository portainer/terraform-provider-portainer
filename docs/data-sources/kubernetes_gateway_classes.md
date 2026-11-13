# Data Source Documentation: `portainer_kubernetes_gateway_classes`

# portainer_kubernetes_gateway_classes
The `portainer_kubernetes_gateway_classes` data source lists the Gateway API GatewayClasses available in a Kubernetes cluster, so a `portainer_kubernetes_gateway` can reference a class that actually exists instead of hard-coding its name per environment.

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

## Example Usage

```hcl
data "portainer_kubernetes_gateway_classes" "all" {
  environment_id = 1
}

output "gateway_class_names" {
  value = [for c in data.portainer_kubernetes_gateway_classes.all.gateway_classes : c.name]
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                              |
|------------------|--------|----------|--------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name              | Type | Description |
|-------------------|------|-------------|
| `gateway_classes` | list | GatewayClasses in the cluster, empty when no Gateway API implementation is installed. Each entry has `name`, `controller_name`, `uid`, `labels` and `annotations`. |
