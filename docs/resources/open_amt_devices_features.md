# 🧬 **Resource Documentation: `portainer_open_amt_devices_features`**

> ⚠️ **Deprecated.** Portainer deprecated OpenAMT in 2.36 and removed it in 2.42, so this resource only works against older Portainer versions. It will be removed in a future major release of the provider.

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
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (endpoint)                  |
| `device_id`      | number | ✅ yes   | ID of the AMT-managed device                                |
| `ider`           | bool   | 🚫 optional | Enable IDE Redirection (`default: false`)                |
| `kvm`            | bool   | 🚫 optional | Enable KVM (Keyboard/Video/Mouse) (`default: false`)     |
| `sol`            | bool   | 🚫 optional | Enable Serial over LAN (`default: false`)                |
| `redirection`    | bool   | 🚫 optional | Enable redirection (`default: false`)                    |
| `user_consent`   | string | 🚫 optional | User consent level (`none`, `all`, `kvmOnly`, etc.) (`default: ""`) |

---

## 📤 Attributes Reference

| Name | Description                                 |
|------|---------------------------------------------|
| `id` | Synthetic ID: `amt-device-features-{device_id}` |
