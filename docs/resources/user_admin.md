# 👑 **Resource Documentation: `portainer_user_admin`**

# portainer_user_admin

The `portainer_user_admin` resource is used **once per Portainer instance** to initialize the built-in `admin` user via the public `/users/admin/init` API.

It is intended for **fresh Portainer installations** and behaves **idempotently**:

- On a *new* instance, it creates the `admin` user.
- On an *already initialized* instance, it detects that and reports success (no error), without changing the existing admin account.

---

## Example Usage

### Initialize admin user on a fresh Portainer instance

```hcl
resource "portainer_user_admin" "init_admin_user" {
  username    = "admin"
  password    = var.portainer_admin_password
  setup_token = var.portainer_setup_token # Portainer 2.45+
}
````

You typically run this **once**, as part of bootstrap (e.g., after deploying Portainer via Terraform).

---

## Lifecycle & Behavior

* The resource first asks the public `GET /users/admin/check` whether an administrator exists. If one does, the resource succeeds and is marked as `initialized = true` without calling the init API.
* Otherwise it calls `POST /users/admin/init` **without authentication** (with `X-Setup-Token` when `setup_token` is set):

  * **200 OK** – the `admin` user is created and the resource is marked as `initialized = true`.
  * **409 Conflict** – an admin user was created in the meantime. The resource still succeeds and is marked as `initialized = true`.

> ✅ This means you can safely keep `portainer_user_admin` in your configuration:
>
> * On first run, it initializes Portainer.
> * On subsequent runs, it becomes a no-op and does **not** fail.

* **Refresh**: the resource stays in state while Portainer reports an administrator (`GET /users/admin/check`), so the plan is clean after the first apply. If the instance loses its administrator (for example after a reset with a fresh data volume), the resource drops out of state and the next apply initializes the admin again.

* **Updates**: changing `username` or `password` after initialization does **not** reconfigure the existing admin user – the init API is a one-time operation by design. The new values are only stored in state.

* **Destroy**: destroying this resource **does not delete** the admin user in Portainer. It only removes the resource from Terraform state.

---

## Arguments Reference

| Name       | Type   | Required | Description                                        |
| ---------- | ------ | -------- | -------------------------------------------------- |
| `username` | string | ✅ yes    | Username for the admin user (typically `"admin"`). |
| `password` | string | ✅ yes    | Password for the admin user. Sensitive value.      |
| `setup_token` | string | 🚫 no | Sent as the `X-Setup-Token` header. Portainer 2.45+ requires it to initialize a fresh instance unless started with `--no-setup-token`; it is printed in the Portainer log (`setup_token=...`) or set with `--setup-token` / `PORTAINER_SETUP_TOKEN`. Sensitive value. |

> 💡 In practice, Portainer expects this to initialize the built-in `admin` account, so using `username = "admin"` is recommended.

---

## Attributes Reference

| Name          | Description                                                                 |
| ------------- | --------------------------------------------------------------------------- |
| `id`          | Identifier of the admin resource (e.g., `"portainer-admin"` or user ID).    |
| `initialized` | Boolean flag indicating whether Portainer reported the admin as initialized |

## Import

This is a singleton resource representing the initial admin user. Import using the admin user's numeric ID (typically `1`):

```shell
terraform import portainer_user_admin.example 1
```

