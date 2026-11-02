# 🧩 **Resource Documentation: `portainer_open_amt_activate`**

> ⚠️ **Deprecated.** Portainer deprecated OpenAMT in 2.36 and removed it in 2.42, so this resource only works against older Portainer versions. It will be removed in a future major release of the provider.

## Overview
The `portainer_open_amt_activate` resource allows you to trigger OpenAMT activation on a Portainer-managed environment (agent endpoint).

---

## 📘 Example Usage

```hcl
resource "portainer_open_amt_activate" "example" {
  environment_id = 1
}
```

---

## ⚙️ Lifecycle & Behavior

This resource activates OpenAMT for the specified environment. It is a one-time action that is triggered on `terraform apply`.

- The endpoint must be agent-based and OpenAMT must be properly configured in Portainer.

---

## 🧾 Arguments Reference

| Name             | Type   | Required | Description                                |
|------------------|--------|----------|--------------------------------------------|
| `environment_id` | number | ✅ yes   | ID of the endpoint where OpenAMT is activated |

---

## 📤 Attributes Reference

| Name | Description                                 |
|------|---------------------------------------------|
| `id` | Always set to `"openamt-{environment_id}"` |

