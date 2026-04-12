# Data Source Documentation: `portainer_kubernetes_crd`

# portainer_kubernetes_crd
The `portainer_kubernetes_crd` data source allows you to list Kubernetes Custom Resource Definitions (CRDs) from a Portainer-managed environment, or retrieve a specific CRD by name.

## Example Usage

### List all CRDs in an environment

```hcl
data "portainer_kubernetes_crd" "all" {
  environment_id = 1
}

output "crd_names" {
  value = [for crd in data.portainer_kubernetes_crd.all.crds : crd.name]
}
```

### Get a specific CRD by name

```hcl
data "portainer_kubernetes_crd" "cert_manager" {
  environment_id = 1
  name           = "certificates.cert-manager.io"
}

output "crd_scope" {
  value = data.portainer_kubernetes_crd.cert_manager.crds[0].scope
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                               |
|------------------|--------|----------|---------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier.                                        |
| `name`           | string | No       | Name of a specific CRD to retrieve. If not set, all CRDs are listed.     |

## Attributes Reference

| Name   | Type | Description                                                                     |
|--------|------|---------------------------------------------------------------------------------|
| `crds` | list | List of CRDs. Each entry has `name`, `group`, `scope`, `creation_date`, `release_name`, `release_namespace`, `release_version`. |
