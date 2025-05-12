# 🔐 **Resource Documentation: `portainer_endpoints_edge_generate_key`**

## Overview
The `portainer_endpoints_edge_generate_key` resource allows administrators to generate a general-purpose Edge key, which can be used to register Portainer Edge Agents.

---

## 📘 Example Usage

```hcl
resource "portainer_endpoints_edge_generate_key" "generated" {}

output "edge_key" {
  value = portainer_endpoints_edge_generate_key.generated.edge_key
}
```

---

## ⚙️ Lifecycle & Behavior

This resource triggers the creation of a new general-purpose Edge key every time the resource is applied. There are no inputs required. The generated key is exposed as an output.

---

## 🧾 Arguments Reference

This resource does not accept any input arguments.

---

## 📤 Attributes Reference

| Name        | Description                                |
|-------------|--------------------------------------------|
| `id`        | Always set to `"portainer-generated-edge-key"` |
| `edge_key`  | The generated Edge key string               |
