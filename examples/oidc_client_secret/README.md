# OIDC client secret

Requires Portainer Business Edition 3.1 or later.

Registers a client of Portainer's OIDC provider and gives it a secret managed on its own. Increase `secret_generation` to regenerate the secret: Portainer issues a new value and keeps the replaced one working for `old_secret_grace_period` (24 hours here), so the application can be given the new value first.

```shell
terraform apply
terraform apply -var secret_generation=2
terraform output -raw client_secret
```

The secret value is returned by Portainer only when it is created or regenerated and is kept in the Terraform state.
