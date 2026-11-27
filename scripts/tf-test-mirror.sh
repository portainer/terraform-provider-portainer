#!/bin/bash
# Mirrors the provider release the upgrade tests (*_UpgradeFromV300,
# *_UpgradeFromSDKv2) apply with, and writes a Terraform CLI configuration that
# installs that provider from the mirror only. Without it every one of those
# tests downloads the release from the registry again, which is most of the
# unit test run time and fails the run when the registry is slow.
#
# Usage: scripts/tf-test-mirror.sh [dir]
#   dir defaults to $TF_TEST_MIRROR_DIR, then .tfmirror in the current
#   directory. The script prints the path of the CLI configuration; point
#   TF_CLI_CONFIG_FILE at it:
#
#   export TF_CLI_CONFIG_FILE="$(scripts/tf-test-mirror.sh)"
#   go test ./internal/
#
# It is idempotent: an existing mirror is reused, so CI can cache the
# directory. OpenTofu reads TF_CLI_CONFIG_FILE too, but resolves
# "portainer/portainer" on registry.opentofu.org: the mirror serves the same
# packages under both hostnames. Without terraform in PATH it mirrors with tofu. Keep the version in sync with previousRelease in
# internal/fw_testhelpers_test.go.

set -euo pipefail

VERSION="${TF_TEST_MIRROR_VERSION:-3.0.0}"
SOURCE="registry.terraform.io/portainer/portainer"
TOFU_SOURCE="registry.opentofu.org/portainer/portainer"
CLI="terraform"
command -v terraform >/dev/null 2>&1 || CLI="tofu"
DIR="${1:-${TF_TEST_MIRROR_DIR:-$PWD/.tfmirror}}"
mkdir -p "${DIR}"
DIR="$(cd "${DIR}" && pwd)"

mirrored() { [ -f "${DIR}/providers/$1/${VERSION}.json" ]; }
if ! mirrored "${SOURCE}" && ! mirrored "${TOFU_SOURCE}"; then
  echo "▶️ Mirror ${SOURCE} ${VERSION} into ${DIR}/providers" >&2
  work="$(mktemp -d)"
  trap 'rm -rf "${work}"' EXIT
  cat >"${work}/main.tf" <<EOF
terraform {
  required_providers {
    portainer = {
      source  = "portainer/portainer"
      version = "${VERSION}"
    }
  }
}
EOF
  # A configuration of our own: the mirror is built from the registry even
  # when the caller already points TF_CLI_CONFIG_FILE at this mirror.
  (cd "${work}" && TF_CLI_CONFIG_FILE=/dev/null "${CLI}" providers mirror "${DIR}/providers" >&2)
fi
# The packages are the same on both registries; copy them to the missing
# hostname (OpenTofu does not follow symbolic links in a mirror).
for pair in "${SOURCE} ${TOFU_SOURCE}" "${TOFU_SOURCE} ${SOURCE}"; do
  read -r have want <<<"${pair}"
  if mirrored "${have}" && ! mirrored "${want}"; then
    mkdir -p "${DIR}/providers/${want}"
    cp -R "${DIR}/providers/${have}/." "${DIR}/providers/${want}/"
  fi
done

cat >"${DIR}/terraformrc" <<EOF
provider_installation {
  filesystem_mirror {
    path    = "${DIR}/providers"
    include = ["${SOURCE}", "${TOFU_SOURCE}"]
  }
  direct {
    exclude = ["${SOURCE}", "${TOFU_SOURCE}"]
  }
}
EOF
echo "${DIR}/terraformrc"
