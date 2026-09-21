# Data Source Documentation: `portainer_omni_service_account`

# portainer_omni_service_account

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Checks whether Omni accepts a service account, before a cluster resource tries to use it.

## Example Usage

```hcl
data "portainer_omni_service_account" "check" {
  endpoint            = var.omni_endpoint
  service_account_key = var.omni_service_account_key
}
```

Reading the result instead of failing:

```hcl
data "portainer_omni_service_account" "check" {
  endpoint            = var.omni_endpoint
  service_account_key = var.omni_service_account_key
  fail_on_error       = false
}

output "omni_credential_valid" {
  value = data.portainer_omni_service_account.check.valid
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint` | string | ✅ yes | URL of the Omni instance, the value of `OMNI_ENDPOINT`. |
| `service_account_key` | string | ✅ yes | Omni service account key, the value of `OMNI_SERVICE_ACCOUNT_KEY`. Marked sensitive. |
| `fail_on_error` | bool | ❌ no | Whether a rejected credential fails the plan. Defaults to `true`. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `valid` | bool | Whether Omni accepted the service account. |
| `error` | string | Why Omni rejected it, empty when accepted. |

`fail_on_error` defaults to `true` so a bad credential stops the plan rather than letting a cluster resource fail part way through provisioning.
