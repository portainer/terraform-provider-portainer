# 🔐 **Resource Documentation: `portainer_resource_control`**

## `portainer_resource_control`

The **`portainer_resource_control`** resource allows you to manage **Portainer ResourceControls**, which define access permissions for Portainer-managed objects such as:

* Docker **Stacks**
* Docker **Secrets**
* (and any other Portainer-managed resource that generates a `ResourceControl` object)

ResourceControls determine which users and teams can access a resource, and whether the resource is public or administrators-only.

This resource can either:

1. **Attach to an existing ResourceControl created by Portainer**
   (e.g. Stacks, Secrets, Containers, etc.)

2. **Directly update a ResourceControl by ID**, using
   `resource_control_id = <id>`
   (common for resources like `portainer_docker_secret`)

---

# 📘 Example Usage

---

## 🔧 **1. ResourceControl for a Docker Secret**

When creating a Docker secret with the `portainer_docker_secret` resource, Portainer automatically generates a ResourceControl.
You can apply permissions using the returned `resource_control_id`:

```hcl
resource "portainer_docker_secret" "example" {
  endpoint_id = 3
  name        = "my_secret"
  data        = base64encode("sensitive-value")
}

resource "portainer_resource_control" "secret_access" {
  resource_control_id = portainer_docker_secret.example.resource_control_id

  type                = 5                # Resource type for Docker secrets
  administrators_only = false
  public              = false
  teams               = [1]
}
```

---

## 📦 **2. ResourceControl for a Docker Stack**

```hcl
resource "portainer_stack" "standalone" {
  name            = "my-stack"
  deployment_type = "standalone"
  method          = "file"
  endpoint_id     = 3

  stack_file_path = "${path.module}/docker-compose.yml"
}

resource "portainer_resource_control" "stack_access" {
  resource_id         = portainer_stack.standalone.id
  type                = 6                  # Stack
  administrators_only = false
  public              = false
  teams               = [8]
  users               = []
}
```

---

# ⚙️ Lifecycle & Behavior

---

### **Create**

* `create` does **not** create a new ResourceControl.
* Instead, it acts as an alias for `update`.
* For resources created via Portainer (Stacks, Secrets…), Portainer automatically creates a ResourceControl when the resource is created.
* Terraform then updates that existing ResourceControl.

---

### **Update**

Changing any of these fields triggers an update:

* `teams`
* `users`
* `public`
* `administrators_only`

For `resource_control_id`-based resources, no API lookup is performed — Terraform uses the ID directly.

---

### **Delete**

Deletes the ResourceControl:

```http
DELETE /resource_controls/{id}
```

If the resource was already deleted upstream, a HTTP `404` is treated as successful removal.

---

### **Changing `resource_id` or `type`**

This forces recreation of the Terraform resource.

---

# 🧩 Arguments Reference

| Name                  | Type         | Required | Description                                                             |
| --------------------- | ------------ | -------- | ----------------------------------------------------------------------- |
| `resource_control_id` | number       | optional | Direct ID of the existing Portainer ResourceControl                     |
| `resource_id`         | string       | optional | ID of the Portainer-managed resource (stack ID)                         |
|`type`                 |number        | optional | Resource type. See full list below.                                     |
| `administrators_only` | bool         | optional | Restrict access to administrators only. Default: `false`                |
| `public`              | bool         | optional | Make the resource public. Default: `false`                              |
| `teams`               | list(number) | optional | List of team IDs allowed to access this resource                        |
| `users`               | list(number) | optional | List of user IDs allowed to access this resource                        |
| `sub_resource_ids`    | list(string) | optional | IDs of sub-resources covered by the same control, such as the services and volumes of a stack. Only used when this resource creates the control. Changing it forces a new resource. |

> When `resource_control_id` is provided, the resource is controlled *directly*, without relying on lookup via `type` + `resource_id`.

### Supported resource types for `type`

Portainer uses these numeric identifiers for ResourceControl types:

- **1** – Container  
- **2** – Service  
- **3** – Volume  
- **4** – Network  
- **5** – Secret  
- **6** – Stack  
- **7** – Config  
- **8** – Swarm  
- **9** – Endpoint  
- **10** – Registry  
- **11** – Team  
- **12** – User  
- **13** – Settings  
- **14** – Edge Group  
- **15** – Edge Stack  
- **16** – Kubernetes ConfigMap  
- **17** – Kubernetes Secret  
- **18** – Kubernetes PersistentVolumeClaim  
- **19** – Kubernetes Application  

---


## How the control is resolved

Portainer creates a resource control implicitly for objects deployed through it, so this resource adopts an existing control whenever it can find one:

1. `resource_control_id` set — that control is managed directly.
2. Otherwise the control is looked up from the resource. Portainer only exposes this for stacks (`type = 6`).
3. If neither yields a control, one is created through `POST /resource_controls`. This is what makes types other than stacks — containers, services, volumes, networks, secrets, configs, custom templates — usable at all.

Portainer has no `GET /resource_controls/{id}`, so a control created by step 3 cannot be read back. For those types the attributes in state stay as configured and drift is not detected; a control that Portainer resolves (step 2) is refreshed normally.

# 📥 Attributes Reference

| Attribute             | Description                                       |
| --------------------- | ------------------------------------------------- |
| `id`                  | ID of the ResourceControl                         |
| `resource_control_id` | Same ID stored as attribute (useful for chaining) |
