# ⚙️ **Resource Documentation: `portainer_open_amt_devices_action`**

## Overview
The `portainer_open_amt_devices_action` resource allows administrators to execute an out-of-band action (such as `poweron`, `poweroff`, `reset`) on an Intel AMT-managed device through Portainer.

---

## 📘 Example Usage

```hcl
resource "portainer_open_amt_devices_action" "reboot" {
  environment_id = 3
  device_id      = 5
  action         = "poweron"
}
```

---

## ⚙️ Lifecycle & Behavior

This resource performs a **single action** on apply. It is not updatable, and `terraform destroy` only removes it from state — the action is not reverted.

---

## 🧾 Arguments Reference

| Name             | Type   | Required | Description                                                    |
|------------------|--------|----------|----------------------------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the Portainer environment (endpoint)                     |
| `device_id`      | number | ✅ yes   | ID of the device within the environment                        |
| `action`         | string | ✅ yes   | Action to execute (`poweron`, `poweroff`, `reset`, etc.)       |

---

## 📤 Attributes Reference

| Name | Description                                                  |
|------|--------------------------------------------------------------|
| `id` | Synthetic ID: `openamt-device-{deviceId}-action-{action}`    |
