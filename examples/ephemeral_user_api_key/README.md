# Ephemeral `portainer_user_api_key`

Issues a Portainer API key for one run and revokes it when the run ends. The key is handed to a
second, aliased `portainer` provider configuration and never written to the plan or the state.

Requires Terraform >= 1.10 or OpenTofu >= 1.11. See
[docs/ephemeral-resources/user_api_key.md](../../docs/ephemeral-resources/user_api_key.md).

```shell
terraform init
terraform apply -var portainer_url=https://portainer.example.com/api -var portainer_user=admin -var portainer_user_id=1
```
