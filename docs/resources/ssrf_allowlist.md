# Resource Documentation: `portainer_ssrf_allowlist`

# portainer_ssrf_allowlist

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Manages the list of URLs Portainer is allowed to make outbound proxy requests to, and how strictly the list is applied. This is Portainer's server-side request forgery (SSRF) control.

## Example Usage

```hcl
resource "portainer_ssrf_allowlist" "main" {
  mode = "enforce"

  entries = [
    "https://hooks.example.com",
    "https://registry.example.com",
  ]
}
```

Rolling it out in audit mode first:

```hcl
resource "portainer_ssrf_allowlist" "main" {
  mode    = "audit"
  entries = ["https://hooks.example.com"]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `mode` | string | ✅ yes | How the list is applied: `off`, `audit` or `enforce`. |
| `entries` | list(string) | ✅ yes | URLs Portainer may make outbound proxy requests to. The list is authoritative: a URL missing from it is removed. |

### Modes

| Value | Behaviour |
|-------|-----------|
| `off` | Outbound requests are not checked against the list at all. |
| `audit` | Requests that are not on the list are recorded but still made. |
| `enforce` | Requests that are not on the list are blocked. |

Portainer stores the mode as an integer (`0`, `1`, `2`). The resource takes the name instead, so a plan says what the change actually does. A mode the provider does not recognise fails the read rather than being folded onto `off`, which would report the control as disabled when it is not.

`mode` has no default. An empty `entries` list under `enforce` blocks every outbound proxy request, so which mode applies is worth stating deliberately rather than inheriting.

There is exactly one allow list in Portainer today, so the resource has no argument to select one. Declaring two of these resources makes each apply overwrite the other.

Portainer has no endpoint to remove the allow list. Destroying the resource stops managing it and leaves the control in place - turning off a security control is a decision to make explicitly, not a side effect of removing it from a configuration.

## Import

```bash
terraform import portainer_ssrf_allowlist.main portainer-ssrf-allowlist
```
