#!/bin/bash
# Manifest-based Kubernetes resources compare the fields their manifest sets
# with the live object: a change made outside Terraform has to show up in the
# plan and be put back by apply, while what Kubernetes adds itself (defaults,
# status, canonical quantities) must not. A changed manifest updates the
# object in place instead of recreating it.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

ENDPOINT_ID="${ENDPOINT_ID:-4}"
export TF_VAR_endpoint_id="${ENDPOINT_ID}"
K8S="/endpoints/${ENDPOINT_ID}/kubernetes"

# A strategic merge patch, like kubectl: it changes one container in place.
patch() { api PATCH "${K8S}$1" -H "Content-Type: application/strategic-merge-patch+json" -d "$2" >/dev/null; }

tf_prepare

step "Apply a ConfigMap and a Deployment"
terraform apply -auto-approve
expect_plan 0 "what Kubernetes adds itself is not drift"

step "Change the ConfigMap outside Terraform"
patch "/api/v1/namespaces/default/configmaps/e2e-drift-config" '{"data":{"log_level":"debug"}}'
expect_plan 2 "the changed ConfigMap shows up in the plan"
expect_plan_contains "log_level: debug" "the plan shows the live value"
terraform apply -auto-approve
[ "$(api GET "${K8S}/api/v1/namespaces/default/configmaps/e2e-drift-config" | jq -r '.data.log_level')" = "info" ] ||
  fail "apply did not put the authored value back"
expect_plan 0 "the ConfigMap converged again"

step "Scale the Deployment outside Terraform"
# Replicas belong to HPAs and portainer_kubernetes_deployment_scale.
patch "/apis/apps/v1/namespaces/default/deployments/e2e-drift-app" '{"spec":{"replicas":2}}'
expect_plan 0 "a changed replica count is not drift"

step "Change the Deployment's image outside Terraform"
patch "/apis/apps/v1/namespaces/default/deployments/e2e-drift-app" \
  '{"spec":{"template":{"spec":{"containers":[{"name":"web","image":"nginx:1.28-alpine"}]}}}}'
expect_plan 2 "the changed image shows up in the plan"
expect_plan_contains "image: nginx:1.28-alpine" "the plan shows the live image"
terraform apply -auto-approve
expect_plan 0 "the Deployment converged again"

# A changed manifest updates the object in place: the same object (UID), a
# rollout of the Deployment, and the replica count left to whoever scaled it.
uid() { api GET "${K8S}$1" | jq -r '.metadata.uid'; }
APP="/apis/apps/v1/namespaces/default/deployments/e2e-drift-app"
CONFIG="/api/v1/namespaces/default/configmaps/e2e-drift-config"
APP_UID="$(uid "${APP}")"
CONFIG_UID="$(uid "${CONFIG}")"

step "Change the image in the manifest"
terraform apply -auto-approve -var=image=nginx:1.30-alpine
[ "$(uid "${APP}")" = "${APP_UID}" ] || fail "the Deployment was recreated instead of updated"
live="$(api GET "${K8S}${APP}")"
[ "$(jq -r '.spec.template.spec.containers[0].image' <<<"${live}")" = "nginx:1.30-alpine" ] ||
  fail "the new image did not reach the Deployment"
[ "$(jq -r '.spec.replicas' <<<"${live}")" = "2" ] || fail "the update reset the replica count"
expect_plan 0 "the updated Deployment converges" -var=image=nginx:1.30-alpine

step "Add and remove a ConfigMap key"
terraform apply -auto-approve -var=image=nginx:1.30-alpine -var=with_feature=true
[ "$(api GET "${K8S}${CONFIG}" | jq -r '.data.feature')" = "on" ] || fail "the added key is missing"
terraform apply -auto-approve -var=image=nginx:1.30-alpine
[ "$(api GET "${K8S}${CONFIG}" | jq -r '.data.feature')" = "null" ] || fail "the removed key is still there"
[ "$(uid "${CONFIG}")" = "${CONFIG_UID}" ] || fail "the ConfigMap was recreated instead of updated"
expect_plan 0 "the updated ConfigMap converges" -var=image=nginx:1.30-alpine

terraform destroy -auto-approve -var=image=nginx:1.30-alpine
echo "✅ Manifest resources show content drift and converge without false positives."
