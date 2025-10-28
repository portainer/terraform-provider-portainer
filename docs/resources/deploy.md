# 🧠 **Resource Documentation: `portainer_deploy`**

# portainer_deploy

The `portainer_deploy` resource allows you to perform automated **service image updates** and **stack environment variable synchronization** for stacks managed by **Portainer**, supporting both **Docker Swarm** and **Docker Standalone** deployments.

It’s designed for use in CI/CD pipelines where you need to automatically roll out new versions (image tags) of services, optionally update environment variables in the stack, and perform **force updates** when necessary.

---

## Example Usage
- [Example Terraform/OpenTofu code with also `.gitlab-ci.yml` on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/deployment)

### Deploy a new version in Docker Swarm

```hcl
resource "portainer_deploy" "swarm_deploy" {
  endpoint_id     = 1
  stack_name      = "my-swarm-stack"
  stack_env_var   = "REVISION"
  services_list   = "web,api"
  revision        = "1.29"
  update_revision = true
  force_update    = true
  wait            = 15
}
```

### Deploy a new version in Docker Standalone

```hcl
resource "portainer_deploy" "standalone_deploy" {
  endpoint_id     = 1
  stack_name      = "nginx"
  stack_env_var   = "VERSION"
  services_list   = "web"
  revision        = "1.29"
  update_revision = true
  force_update    = false
  wait            = 10
}
```

## 🚀 Example Usage of full automation deployment steps for Portainer
### Terraform/OpenTofu code:
```hcl
### Pull image for new version of the service
resource "portainer_docker_image" "pull" {
  endpoint_id = 1
  image       = "nginx:1.29"
}

### Deploy new version of the service
resource "portainer_deploy" "deploy" {
  depends_on = [portainer_docker_image.pull]

  endpoint_id     = 1
  stack_name      = "nginx"
  services_list   = "web"
  revision        = "1.29"
  update_revision = true
  stack_env_var   = "VERSION"
  force_update    = true
  wait            = 10
}

### Execute command inside the updated container/service
resource "portainer_container_exec" "exec" {
  depends_on = [portainer_deploy.deploy]

  endpoint_id   = 1
  service_name  = "web"
  command       = "nginx -s reload"
  user          = "root"
}

### Check service or container status after deployment
resource "portainer_check" "check" {
  depends_on = [portainer_container_exec.exec]

  endpoint_id     = 1
  stack_name      = "nginx"
  services_list   = "web"
  revision        = "1.29"
  desired_state   = "running"
  max_retries     = 3
  wait            = 10
  wait_between_checks = 5
}

### Optional — Output results from each step
output "deploy_log" {
  value = portainer_deploy.deploy.output
}

output "exec_output" {
  value = portainer_container_exec.exec.output
}

output "check_result" {
  value = portainer_check.check.output
}
```

### Example of .gitlab-ci.yml:
```yml
image: hashicorp/terraform  # or use ghcr.io/opentofu/opentofu if you can use tofu commands instead of terraform

stages:
  - test
  - plan
  - deploy

before_script:
  - rm -rf terraform.tfstate
  - rm -rf terraform.tfstate.backup

test:
  stage: test
  script:
    - terraform init
    - terraform check
    - terraform validate
  artifacts:
    paths:
      - .terraform.lock.hcl
      - .terraform
    expire_in: 1h

plan:
  stage: plan
  script:
    - terraform plan -target=portainer_docker_image.pull -out=plan-pull.tfplan
    - terraform plan -target=portainer_deploy.deploy -out=plan-deploy.tfplan
    - terraform plan -target=portainer_container_exec.exec -out=plan-exec.tfplan
    - terraform plan -target=portainer_check.check -out=plan-check.tfplan
  artifacts:
    paths:
      - plan*
      - .terraform.lock.hcl
      - .terraform
    expire_in: 1h

deploy:
  stage: deploy
  only: master
  when: manual
  script:
    - terraform apply -auto-approve plan-pull.tfplan
    - terraform apply -auto-approve plan-deploy.tfplan
    - terraform apply -auto-approve plan-exec.tfplan
    - terraform apply -auto-approve plan-check.tfplan
```

### Example of output in terminal between deploy/apply steps:
```
portainer_docker_image.pull: Creating...
portainer_docker_image.pull: Creation complete after 2s [id=3-nginx:1.29]
portainer_deploy.deploy: Creating...
portainer_deploy.deploy: Creation complete after 2s [id=deploy-1761672889]
portainer_container_exec.exec: Creating...
portainer_container_exec.exec: Creation complete after 0s [id=408add647b345365cf42366b4964479d22cb57e5ee87d8e8f680c052603a0eb4]
portainer_check.check: Creating...
portainer_check.check: Still creating... [00m10s elapsed]
portainer_check.check: Creation complete after 10s [id=check-1761672899]

Apply complete! Resources: 5 added, 0 changed, 0 destroyed.

Outputs:

check_output = <<EOT
Starting container check for stack "nginx" with revision "1.29"
Waiting 10 seconds before first check...
Docker Standalone detected — using container check logic.
DEBUG: checking container="nginx-web-1" (image="nginx:1.29", state="running")
Container "nginx-web-1" OK — revision "1.29", state "running"

EOT
deploy_output = <<EOT
Docker Standalone detected — using standalone stack update logic.
Standalone stack "nginx" updated with VERSION="1.29"

EOT
exec_output = <<EOT
<2025/10/28 17:34:49 [notice] 27#27: signal process started

EOT
```

---

## ⚙️ Lifecycle & Behavior

This resource is **stateless** — it triggers a one-time deployment or update action on the stack and does **not** store persistent state in Portainer.

When executed:

1. Detects whether the environment is **Swarm** or **Standalone**.
2. Updates all listed services to the provided image tag (`revision`).
3. Optionally updates a stack environment variable (`stack_env_var`) to the same revision if `update_revision = true`.
4. Optionally performs a **force update** (`force_update = true`) to trigger immediate service refresh, pulling new images.
5. Waits for the configured `wait` duration before applying a force update.

> 💡 **Pro Tip:** Combine with `portainer_check` to verify that containers are running with the updated version after deployment.

---

## 📥 Arguments Reference

| Name              | Type   | Required                      | Description                                                                                   |
| ----------------- | ------ | ----------------------------- | --------------------------------------------------------------------------------------------- |
| `endpoint_id`     | int    | ✅ yes                         | ID of the Portainer environment (endpoint) where the stack resides.                           |
| `stack_name`      | string | ✅ yes                         | Name of the stack to be updated.                                                              |
| `stack_env_var`   | string | ✅ yes                         | Name of the stack environment variable to update with the new `revision`.                     |
| `revision`        | string | ✅ yes                         | Target image tag/revision (e.g. `"1.29"`) to deploy across selected services.                 |
| `services_list`   | string | ✅ yes                         | Comma-separated list of service names (without stack prefix) to update. Example: `"web,api"`. |
| `update_revision` | bool   | 🚫 optional (default `true`)  | If true, also updates the environment variable `stack_env_var` with the provided `revision`.  |
| `force_update`    | bool   | 🚫 optional (default `false`) | If true, triggers Portainer’s `/forceupdateservice` endpoint after updating service images.   |
| `wait`            | int    | 🚫 optional (default `30`)    | Seconds to wait before performing a force update (used only when `force_update = true`).      |

---

## 📤 Attributes Reference

| Name     | Description                                                                                              |
| -------- | -------------------------------------------------------------------------------------------------------- |
| `id`     | Auto-generated ID of the deployment run (stateless).                                                     |
| `output` | Verbose textual output of the deployment process (includes update actions, service names, and warnings). |

---

## 🧩 Example with Outputs

```hcl
output "deploy_log" {
  value = portainer_deploy.swarm_deploy.output
}
```

**Example Output:**

```
Docker Swarm detected — using swarm update logic.
Service "my-swarm-stack_web" updated to "nginx:1.29"
Force update of "my-swarm-stack_web" succeeded
Stack "my-swarm-stack" REVISION updated to "1.29"
```

---

## 🔄 Execution Flow Overview

| Step                        | Description                                                                                          |
| --------------------------- | ---------------------------------------------------------------------------------------------------- |
| 1️⃣ Detect Environment      | The resource calls `/endpoints/{id}/docker/swarm` to determine if the target is Swarm or Standalone. |
| 2️⃣ Update Services         | For each listed service, the image tag is replaced with the specified `revision`.                    |
| 3️⃣ Update Environment      | If `update_revision` is enabled, the stack variable `stack_env_var` is updated.                      |
| 4️⃣ Force Update (optional) | If `force_update` is enabled, the `/forceupdateservice` API is called for each service.              |
| 5️⃣ Output Summary          | A detailed deployment log is stored in the computed `output` attribute.                              |

---

## 🧠 Summary

| Feature      | Description                                                                                                            |
| ------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Mode         | Works in both **Swarm** and **Standalone** modes                                                                       |
| Purpose      | Update service image tags and optionally stack environment variables                                                   |
| Behavior     | Stateless — runs once per `apply`                                                                                      |
| Use Case     | Continuous delivery automation, image version rollouts, or blue-green updates                                          |
| Dependencies | None required, but typically follows an image pull (`portainer_docker_image`) and precedes a check (`portainer_check`) |
