# KubeSolo Edge environment via cloud-init

Requires Portainer Business Edition 3.0 or later.

Creates an Edge Agent environment with `portainer_environment` and renders cloud-init user data that installs [KubeSolo](https://kubesolo.io/) on the machine's first boot and joins it to that environment. The environment's `edge_id` and `edge_key` are passed to the KubeSolo installer as `KUBESOLO_PORTAINER_EDGE_ID` and `KUBESOLO_PORTAINER_EDGE_KEY`:

```sh
curl -sfL https://get.kubesolo.io | KUBESOLO_PORTAINER_EDGE_ID=... KUBESOLO_PORTAINER_EDGE_KEY=... sudo -E sh
```

Feed the sensitive `kubesolo_user_data` output to whatever creates the machine (the `user_data` of a cloud instance, the cloud-init drive of a VM). Once the agent checks in, Portainer turns the environment into a Kubernetes Edge Agent environment, and `portainer_kubesolo_config` reports the running KubeSolo version and configuration.

Portainer's own install over SSH (Environments > Add environment > KubeSolo > Install KubeSolo on many machines) is not modelled by the provider, as it is an asynchronous background job.

```sh
terraform init
terraform apply -var portainer_url=https://portainer.example.com:9443 \
  -var portainer_api_key=... \
  -var portainer_edge_url=https://portainer.example.com:9443
terraform output -raw kubesolo_user_data > user-data.yaml
```
