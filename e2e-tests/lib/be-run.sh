#!/bin/bash
# Business Edition e2e tests, against the instance e2e-tests/lib/be-setup.sh
# started (its variables have to be in the environment). The license goes in
# first, through portainer_licenses, since the other tests need it; each test
# then applies, has to converge, and is destroyed.
#
# Needs TF_VAR_license_key (the GitHub secret PORTAINER_BE_LICENSE).

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

run_test be_license license
run_test be_settings_kube apply_converge_destroy
run_test be_cloud_credentials cloud_credentials
run_test be_endpoint_settings apply_converge_destroy
run_test be_container_webhook container_webhook

echo "✅ Business Edition tests passed."
