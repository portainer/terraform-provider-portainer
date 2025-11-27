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
  username = "admin"
  password = var.portainer_admin_password
}
````

You typically run this **once**, as part of bootstrap (e.g., after deploying Portainer via Terraform).

---

## Lifecycle & Behavior

* The resource calls `POST /users/admin/init` **without authentication**.
* If Portainer responds with:

  * **200 OK** – the `admin` user is created and the resource is marked as `initialized = true`.
  * **409 Conflict** – an admin user already exists. The resource still succeeds and is marked as `initialized = true`.

> ✅ This means you can safely keep `portainer_user_admin` in your configuration:
>
> * On first run, it initializes Portainer.
> * On subsequent runs, it becomes a no-op and does **not** fail.

* **Updates**: changing `username` or `password` after initialization does **not** reconfigure the existing admin user – the init API is a one-time operation by design. Terraform will show a diff, but the API call will effectively remain a no-op on an already initialized instance.

* **Destroy**: destroying this resource **does not delete** the admin user in Portainer. It only removes the resource from Terraform state.

---

## Arguments Reference

| Name       | Type   | Required | Description                                        |
| ---------- | ------ | -------- | -------------------------------------------------- |
| `username` | string | ✅ yes    | Username for the admin user (typically `"admin"`). |
| `password` | string | ✅ yes    | Password for the admin user. Sensitive value.      |

> 💡 In practice, Portainer expects this to initialize the built-in `admin` account, so using `username = "admin"` is recommended.

---

## Attributes Reference

| Name          | Description                                                                 |
| ------------- | --------------------------------------------------------------------------- |
| `id`          | Identifier of the admin resource (e.g., `"portainer-admin"` or user ID).    |
| `initialized` | Boolean flag indicating whether Portainer reported the admin as initialized |

