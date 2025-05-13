# 🚀 **Resource Documentation: `portainer_edge_update_schedules`**

## Overview
The `portainer_edge_update_schedules` resource allows you to schedule update or rollback procedures for Edge agents in Portainer.

---

## 📘 Example Usage

```hcl
resource "portainer_edge_update_schedules" "example" {
  name            = "scheduled-edge-update"
  agent_image     = "portainer/agent:2.19.0"
  updater_image   = "portainer/portainer-updater:2.19.0"
  registry_id     = 1
  scheduled_time  = "2025-05-10T10:00:00Z"
  group_ids       = [1]
  type            = 0 # 0 = update, 1 = rollback
}
```

---

## ⚙️ Lifecycle & Behavior
- **Create** uses `POST /edge_update_schedules`
- **Read** uses `GET /edge_update_schedules/{id}`
- **Update** uses `POST /edge_update_schedules/{id}`
- **Delete** uses `DELETE /edge_update_schedules/{id}`

---

## 🧾 Arguments Reference

| Name             | Type   | Required | Description                                                  |
|------------------|--------|----------|--------------------------------------------------------------|
| `name`           | string | ✅ yes   | Name of the update schedule                                  |
| `agent_image`    | string | ✅ yes   | Docker image to update the Edge agents                       |
| `updater_image`  | string | ✅ yes   | Docker image for the updater component                       |
| `registry_id`    | number | ✅ yes   | ID of the registry to pull images from                       |
| `scheduled_time` | string | ✅ yes   | Time in RFC3339 format (e.g., `2025-05-10T10:00:00Z`)        |
| `group_ids`      | list   | ✅ yes   | List of Edge group IDs to apply the update to               |
| `type`           | number | ✅ yes   | `0 = update`, `1 = rollback`                                 |

---

## 📤 Attributes Reference

| Name | Description                     |
|------|---------------------------------|
| `id` | ID of the update schedule in Portainer |
