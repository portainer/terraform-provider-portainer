# 🔐 Resource Documentation: `portainer_sshkeygen`

## Overview
The `portainer_sshkeygen` resource allows you to generate a public/private SSH keypair using the Portainer API.

Useful for secure deployments and automations where keys must be generated dynamically in a reproducible and managed way.

---

## 📘 Example Usage

```hcl
resource "portainer_sshkeygen" "generated" {}

output "public_key" {
  value = portainer_sshkeygen.generated.public
}
```

---

## ⚙️ Lifecycle & Behavior
- **Create**: generates SSH keypair via `POST /sshkeygen`

---

## 🧾 Arguments Reference

This resource accepts no input arguments.

---

## 📤 Attributes Reference

> Note: The `private` key is marked as sensitive, so it won't be shown unless you run `terraform output --sensitive`.

| Name      | Description                     |
|-----------|---------------------------------|
| `public`  | The generated SSH public key    |
| `private` | The generated SSH private key (sensitive) |
| `id`      | Arbitrary ID derived from key length |
