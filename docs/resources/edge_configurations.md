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
- **Update** sends/updates the state using `PUT /edge_configurations/{id}`.
- **Delete** removes the configuration via `DELETE /edge_configurations/{id}`.
- **Read** retrieves metadata and synchronizes state with Portainer using `GET /edge_configurations/{id}`.

---

## 🧾 Arguments Reference

| Name             | Type   | Required | Description                                                             |
|------------------|--------|----------|-------------------------------------------------------------------------|
| `name`           | string | ✅ yes   | Name of the Edge Configuration                                          |
| `type`           | string | ✅ yes   | Type of configuration (e.g., `general` or `device`)                     |
| `edge_group_ids` | list(number) | ✅ yes | List of Edge Group IDs                                              |
| `file_path`      | string | ✅ yes   | Path to the configuration file to upload                                |
| `category`       | string | 🚫 no    | Optional category                                                       |
| `base_dir`       | string | 🚫 no    | Optional base directory for the configuration                           |

---

## 📤 Attributes Reference

| Name | Description                             |
|------|-----------------------------------------|
| `id` | Synthetic ID based on uploaded file name |
