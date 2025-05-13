# 🧬 **Resource Documentation: `portainer_open_amt_devices_features`**

## Overview
The `portainer_open_amt_devices_features` resource enables or configures remote management features (KVM, IDER, SOL, etc.) on a device managed via Intel AMT, through Portainer.

---

## 📘 Example Usage

```hcl
resource "portainer_open_amt_devices_features" "example" {
  environment_id = 1
  device_id      = 42

  ider         = true
  kvm          = true
  sol          = true
  redirection  = true
  user_consent = "kvmOnly"
}
```

---

## ⚙️ Lifecycle & Behavior
- This resource performs a one-time configuration of AMT device features.
- Changing a value will cause a new API call on `terraform apply`.

---

## 🧾 Arguments Reference

| Name           | Type   | Required | Description                                                   |
|----------------|--------|----------|---------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (endpoint)                    |
| `device_id`      | number | ✅ yes   | ID of the AMT-managed device                                  |
| `ider`           | bool   | 🚫 no    | Enable IDE Redirection                                        |
| `kvm`            | bool   | 🚫 no    | Enable KVM (Keyboard/Video/Mouse)                             |
| `sol`            | bool   | 🚫 no    | Enable Serial over LAN                                        |
| `redirection`    | bool   | 🚫 no    | Enable redirection                                            |
| `user_consent`   | string | 🚫 no    | User consent level (`none`, `all`, `kvmOnly`, etc.)           |

---

## 📤 Attributes Reference

| Name | Description                                 |
|------|---------------------------------------------|
| `id` | Synthetic ID: `amt-device-features-{device_id}` |
