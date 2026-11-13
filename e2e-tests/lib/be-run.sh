#!/bin/bash
# Business Edition e2e tests, against the instance e2e-tests/lib/be-setup.sh
# started (its variables have to be in the environment). The license goes in
# first, through portainer_licenses, since the other tests need it; each test
# then applies, has to converge, and is destroyed.
#
# Needs TF_VAR_license_key (the GitHub secret PORTAINER_BE_LICENSE), and for
# the Kubernetes tests TF_VAR_k8s_environment_id from be-k8s-setup.sh. The 3.0
# tests run when PORTAINER_VERSION is 3.x.

set -euo pipefail
cd "$(dirname "$0")/.."

[ -n "${TF_VAR_license_key:-}" ] || {
  echo "❌ TF_VAR_license_key is not set"
  exit 1
}

# run_test <dir> <commands...>: runs the commands in the test directory with
# the shared helpers, in a subshell so a failure only cleans up that test.
run_test() {
  local dir="$1"
  shift
  echo "▶️ ${dir}"
  (
    cd "${dir}"
    # shellcheck source=e2e-tests/lib/e2e.sh
    source ../lib/e2e.sh
    tf_prepare >/dev/null
    "$@"
  )
}

apply_converge_destroy() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  terraform destroy -auto-approve -input=false
}

license() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  # key_wo: the key must not be in the state.
  if terraform state pull | grep -qF "${TF_VAR_license_key}"; then
    echo "❌ the license key is in the Terraform state"
    return 1
  fi
  # Kept: the remaining tests need the license.
}

cloud_credentials() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  terraform apply -auto-approve -input=false -var=name=e2e-aws-renamed
  expect_plan 0 "the renamed credentials converge" -var=name=e2e-aws-renamed
  terraform destroy -auto-approve -input=false -var=name=e2e-aws-renamed
}

container_webhook() {
  docker rm -f e2e-be-webhook-target >/dev/null 2>&1 || true
  TF_VAR_container_id="$(docker run -d --name e2e-be-webhook-target busybox:1.36 sleep 3600)"
  export TF_VAR_container_id
  apply_converge_destroy
  docker rm -f e2e-be-webhook-target >/dev/null
}

alerting() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  local adopted custom template fields='{promqlTemplate, tiers, name}'
  adopted="$(terraform output -raw adopted_id)"
  custom="$(terraform output -raw custom_id)"
  template="$(api GET "/observability/alerting/rules/${adopted}" | jq -c "${fields}")"
  step "an update keeps what the configuration does not set"
  terraform apply -auto-approve -input=false -var threshold=77
  expect_plan 0 "the updated rules converge" -var threshold=77
  [ "$(api GET "/observability/alerting/rules/${adopted}" | jq -c "${fields}")" = "${template}" ] ||
    fail "the update of the adopted rule changed fields it does not configure"
  terraform destroy -auto-approve -input=false -var threshold=77
  step "destroy deletes the custom rule only"
  api GET "/observability/alerting/rules/${adopted}" >/dev/null || fail "the adopted rule was deleted"
  if api GET "/observability/alerting/rules/${custom}" >/dev/null 2>&1; then
    fail "the custom rule was not deleted"
  fi
}

gateway() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  step "a listener change updates the gateway in place"
  terraform apply -auto-approve -input=false -var port=8080
  expect_plan 0 "the updated gateway converges" -var port=8080
  terraform destroy -auto-approve -input=false -var port=8080
}

d2k() {
  terraform apply -auto-approve -input=false
  bash ../lib/expect-converged.sh
  step "a D2K environment imports by its environment ID"
  terraform state rm portainer_d2k_environment.e2e >/dev/null
  terraform import -input=false portainer_d2k_environment.e2e "$(api GET /endpoints | jq -r '.[] | select(.Name == "e2e-d2k") | .Id')"
  expect_plan 0 "the imported D2K environment converges"
  terraform destroy -auto-approve -input=false
}

run_test be_license license
run_test be_settings_kube apply_converge_destroy
run_test be_cloud_credentials cloud_credentials
run_test be_endpoint_settings apply_converge_destroy
run_test be_container_webhook container_webhook
run_test be_alerting alerting
run_test be_gitops apply_converge_destroy
if [ -n "${TF_VAR_k8s_environment_id:-}" ]; then
  run_test be_scoped_kubeconfig apply_converge_destroy
fi

case "${PORTAINER_VERSION:-}" in
  3.*)
    [ -n "${TF_VAR_k8s_environment_id:-}" ] || {
      echo "❌ the 3.0 tests need TF_VAR_k8s_environment_id (e2e-tests/lib/be-k8s-setup.sh)"
      exit 1
    }
    run_test be_data_sources_30 apply_converge_destroy
    run_test be_gateway gateway
    run_test be_reconcile apply_converge_destroy
    run_test be_d2k d2k
    ;;
esac

echo "✅ Business Edition tests passed."
