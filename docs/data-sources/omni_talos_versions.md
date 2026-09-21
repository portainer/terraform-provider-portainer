# Data Source Documentation: `portainer_omni_talos_versions`

# portainer_omni_talos_versions

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Lists the Talos versions a credential can provision.

## Example Usage

```hcl
data "portainer_omni_talos_versions" "supported" {
  credential_id = portainer_cloud_credentials.omni.id
}

output "newest_talos" {
  value = one(reverse(data.portainer_omni_talos_versions.supported.talos_versions))
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `talos_versions` | list(string) | The Talos versions on offer, sorted. |
| `compatibility` | list(object) | For each Talos version, the versions Portainer lists against it. |

### `compatibility`

| Name | Type | Description |
|------|------|-------------|
| `talos_version` | string | A Talos version on offer. |
| `kubernetes_versions` | list(string) | Kubernetes versions Portainer lists against that Talos version. |

Portainer returns this as a map of version to version list. The sorting is done by the provider so the output is stable between plans; Portainer itself does not define an order.
