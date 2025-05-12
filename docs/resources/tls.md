# 🔐 **Resource Documentation: `portainer_tls`**

## Overview
The `portainer_tls` resource allows you to upload TLS-related files (CA, certificate, or private key) to a specified folder within Portainer.

---

## 📘 Example Usage

```hcl
resource "portainer_tls" "upload_cert" {
  certificate = "cert"  # allowed: "ca", "cert", "key"
  folder      = "my-endpoint-folder"
  file_path   = "my-cert.pem"
}
```

---

## ⚙️ Lifecycle & Behavior
- This resource performs a one-time upload of a TLS file.
- There is no way to update or delete the file via Terraform — `terraform destroy` only removes the state.
- Accepted `certificate` values: `ca`, `cert`, `key`.

---

## 🧾 Arguments Reference

| Name         | Type   | Required | Description                                                  |
|--------------|--------|----------|--------------------------------------------------------------|
| `certificate`| string | ✅ yes   | Type of TLS file (`ca`, `cert`, or `key`)                    |
| `folder`     | string | ✅ yes   | Folder name to upload the TLS file into                      |
| `file_path`  | string | ✅ yes   | Local file path to the TLS file being uploaded               |

---

## 📤 Attributes Reference

| Name | Description                                          |
|------|------------------------------------------------------|
| `id` | Set to `upload-{certificate}-{filename}`             |
