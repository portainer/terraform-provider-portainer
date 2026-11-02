#!/bin/bash
# portainer_kubernetes_environment_config: Portainer replaces the environment's
# whole Kubernetes object on update, so the resource has to keep what it does
# not manage, read its settings back and show changes made in the UI.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

ENDPOINT_ID="${ENDPOINT_ID:-4}"
export TF_VAR_endpoint_id="${ENDPOINT_ID}"

kube_config() { api GET "/endpoints/${ENDPOINT_ID}" | jq -c "$1"; }

tf_prepare
before_flags="$(kube_config '.Kubernetes.Flags')"

step "Apply the configuration"
terraform apply -auto-approve
[ "$(kube_config '.Kubernetes.Configuration.UseLoadBalancer')" = "true" ] || fail "UseLoadBalancer was not stored"
[ "$(kube_config '.Kubernetes.Configuration.StorageClasses[0].Name')" = '"local-path"' ] || fail "the storage class was not stored"
[ "$(kube_config '.Kubernetes.Flags')" = "${before_flags}" ] || fail "the detected cluster flags changed: ${before_flags} -> $(kube_config '.Kubernetes.Flags')"
echo "   stored, and the detected cluster flags are untouched"
expect_plan 0 "no diff after apply"

step "Restrict the default namespace in Portainer, outside Terraform"
kube="$(api GET "/endpoints/${ENDPOINT_ID}" | jq -c '.Kubernetes | .Configuration.RestrictDefaultNamespace = true')"
api PUT "/endpoints/${ENDPOINT_ID}" -d "{\"Kubernetes\": ${kube}}" >/dev/null
expect_plan 2 "the change made in Portainer shows up in the plan"
expect_plan_contains "restrict_default_namespace +=.*true -> false" "the plan restores restrict_default_namespace"
terraform apply -auto-approve

step "Import into a fresh state"
reimport portainer_kubernetes_environment_config.e2e
expect_plan 0 "imported configuration needs no change"

terraform destroy -auto-approve
echo "✅ The Kubernetes environment configuration round-trips, shows drift and imports."
