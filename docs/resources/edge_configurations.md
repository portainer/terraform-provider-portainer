# 🌐 **Resource Documentation: `portainer_edge_configurations`**

## Overview
The `portainer_edge_configurations` resource allows you to create, update, read, and delete Edge Configurations in Portainer.
It also optionally allows setting the state of the configuration after creation or update.

---

## 📘 Example Usage

```hcl
resource "portainer_edge_configurations" "example_edge_configuration" {
  name            = "Test Edge Config"
  type            = "general"
  category        = "configuration"
  base_dir        = "/etc/some/path/of/edge/config"                        # optional
  edge_group_ids  = [1]                                                    # optional
  file_path       = "${path.module}/config.zip"
}
```
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/edge_configurations)

---

## ⚙️ Lifecycle & Behavior
- **Create** uploads a configuration file and sets the name, type, and association to edge groups.
- **Update** updates the configuration via `PUT /edge_configurations/{id}` (supports changing `type`, `edge_group_ids`, and `file_path`, and is also triggered when the contents of the file at `file_path` change — see `file_sha256` below).
- **Delete** removes the configuration via `DELETE /edge_configurations/{id}`.
- **Read** retrieves metadata and synchronizes state with Portainer using `GET /edge_configurations/{id}`. The Portainer API does not return file content or any digest, so `file_sha256` is preserved from state and recomputed locally during plan.

> 💡 **File content change detection:** During plan the provider reads `file_path` and computes its SHA256 into the `file_sha256` computed attribute. If the new hash differs from state, Terraform plans an in-place Update — even when `file_path` itself didn't change. This means rewriting the file in place (or `terraform apply` after the file's bytes change) will correctly re-upload it. `file_path` must be readable at plan time.

> ⚠️ **Same-name limitation:** Portainer's `POST /edge_configurations` endpoint does not return the new configuration's ID. To resolve the ID after create, the provider snapshots existing edge configurations matching the requested `name` *before* the POST and picks the new entry that appears afterwards. If multiple entries appear (concurrent writers) the most recently created one is chosen. Because Portainer permits multiple edge configurations with the same `name`, **avoid creating Terraform-managed and out-of-band configurations with identical names** — if a same-name entry is created concurrently with `terraform apply`, the provider may bind to the wrong record. This will be fully resolved once Portainer returns the ID on POST.

---

## 🧾 Arguments Reference

| Name             | Type   | Required | Description                                                             |
|------------------|--------|----------|-------------------------------------------------------------------------|
| `name`           | string | ✅ yes   | Name of the Edge Configuration                                          |
| `type`           | string | ✅ yes   | Type of configuration. Allowed values: `general`, `filename`, or `foldername` |
| `edge_group_ids` | list(number) | ✅ yes | List of Edge Group IDs                                              |
| `file_path`      | string | ✅ yes   | Path to the configuration file to upload                                |
| `category`       | string | 🚫 no    | Optional category. Allowed values: `configuration`, `secret`. Changing forces recreation |
| `base_dir`       | string | 🚫 no    | Optional base directory for the configuration                           |

---

## 📤 Attributes Reference

| Name           | Description                                                                 |
|----------------|-----------------------------------------------------------------------------|
| `id`           | The Edge Configuration ID assigned by Portainer                             |
| `file_sha256`  | SHA256 hex digest of the uploaded file's contents, used to detect in-place file changes between plans |
