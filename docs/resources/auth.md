# 🔐 **Resource Documentation: portainer_auth**

# portainer_auth
The `portainer_auth` resource allows you to authenticate to the Portainer API using a username and password.
It returns a JWT token that can be used for other API calls or debug purposes.

## Example Usage
### Create Backup
```hcl
resource "portainer_auth" "login" {
  username = "admin"
  password = "password"
}

output "jwt_token" {
  value     = portainer_auth.login.jwt
  sensitive = true
}
```
> ✅ Note: This resource does not persist anything in Portainer. It only returns a JWT token that can be used in subsequent API calls.

## Lifecycle & Behavior
- This resource authenticates via the /auth API endpoint using username/password.
- It always re-authenticates on every terraform apply.
- The result is a JWT token stored in state/output.
To use the token:
```hcl
output "jwt_token" {
  value     = portainer_auth.login.jwt
  sensitive = true
}
```

## Arguments Reference

| Name      | Type   | Required | Description                |
|-----------|--------|----------|----------------------------|
| `username`| string | ✅ yes   | Portainer admin username   |
| `password`| string | ✅ yes   | Portainer admin password   |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `jwt` | The JWT token returned from /auth |
