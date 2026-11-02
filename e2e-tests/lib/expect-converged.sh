#!/bin/bash
# Fails unless the configuration in the current directory converged: right
# after an apply, `terraform plan` must have nothing to do. A plan that keeps
# proposing changes means Read and the configuration disagree, which every
# user of the resource would see on every run.
#
# Some resources settle asynchronously (a stack reports active only once
# Portainer has deployed it), so a non-empty plan is retried a few times
# before it fails the test.
#
# Usage, from an e2e test directory after `terraform apply`:
#
#   bash ../lib/expect-converged.sh

set -uo pipefail

ATTEMPTS="${CONVERGE_ATTEMPTS:-3}"
DELAY="${CONVERGE_DELAY:-10}"

plan_out="$(mktemp)"
trap 'rm -f "${plan_out}"' EXIT

for attempt in $(seq 1 "${ATTEMPTS}"); do
  rc=0
  terraform plan -detailed-exitcode -input=false -no-color >"${plan_out}" 2>&1 || rc=$?
  case "${rc}" in
    0)
      echo "✅ $(basename "${PWD}"): converged"
      exit 0
      ;;
    2)
      if [ "${attempt}" -lt "${ATTEMPTS}" ]; then
        echo "   $(basename "${PWD}"): plan not empty yet, retrying in ${DELAY}s (${attempt}/${ATTEMPTS})"
        sleep "${DELAY}"
      fi
      ;;
    *)
      cat "${plan_out}"
      echo "❌ $(basename "${PWD}"): plan failed (exit ${rc})"
      exit 1
      ;;
  esac
done

cat "${plan_out}"
echo "❌ $(basename "${PWD}"): the plan right after apply is not empty, so the configuration does not converge"
exit 1
