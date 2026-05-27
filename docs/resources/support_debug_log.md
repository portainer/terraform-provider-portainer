# 🐞 Resource Documentation: `portainer_support_debug_log`

## Overview
The `portainer_support_debug_log` resource allows administrators to enable or disable the global debug log in Portainer.

This is useful for collecting additional runtime information to help with diagnostics and troubleshooting.

> Currently working only for Portainer BE edition

---

## 📘 Example Usage

```hcl
resource "portainer_support_debug_log" "debug" {
  enabled = true
}
```

---

## ⚙️ Lifecycle & Behavior
- **Create** and **Update**: call `PUT /support/debug_log` with `{ debugLogEnabled: true|false }`
- **Read**: uses `GET /support/debug_log` to confirm current log status
- **Delete**: automatically disables the debug log via `PUT /support/debug_log` with `false`

---

## 🧾 Arguments Reference

| Name      | Type  | Required | Description                                   |
|-----------|-------|----------|-----------------------------------------------|
| `enabled` | bool  | ✅ yes   | Whether the global debug log should be active |

---

## 📤 Attributes Reference

| Name | Description                                   |
|------|-----------------------------------------------|
| `id` | Set to string value of `true` or `false`      |

## Import

This is a singleton toggle resource; its ID is the string value of the `enabled` flag (`true` or `false`). Import using the current state:

```shell
terraform import portainer_support_debug_log.example true
```
