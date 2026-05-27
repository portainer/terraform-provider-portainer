# 🧪 **Resource Documentation: `portainer_settings_experimental`**

## Overview
The `portainer_settings_experimental` resource allows you to manage experimental features in Portainer, such as enabling OpenAI integration.

> ⚠️ This endpoint is intended for administrators only. Use with care.

> Currently working only for Portainer BE edition

---

## 📘 Example Usage

```hcl
resource "portainer_settings_experimental" "example" {
  openai_integration = false
}
```

---

## ⚙️ Lifecycle & Behavior
This resource performs an update of the Portainer experimental settings via the following API call:

```
PUT /settings/experimental
```

If any setting is changed, Terraform will re-apply the configuration during `terraform apply`. 

---

## 🧾 Arguments Reference

| Name                  | Type  | Required | Description                                       |
|-----------------------|-------|----------|---------------------------------------------------|
| `openai_integration`  | bool  | ✅ yes   | Enable or disable the OpenAI integration toggle   |

---

## 🪪 Attributes Reference

| Name  | Description                              |
|-------|------------------------------------------|
| `id`  | Always set to `"portainer-experimental-settings"` |

## Import

The experimental settings are a singleton resource; import using the fixed ID `portainer-experimental-settings`:

```shell
terraform import portainer_settings_experimental.example portainer-experimental-settings
```
