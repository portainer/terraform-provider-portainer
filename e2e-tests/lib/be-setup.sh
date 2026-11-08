#!/bin/bash
# Starts a fresh Portainer Business Edition for the BE e2e tests: initializes
# the admin with the setup token from the log, creates an API key and a local
# Docker environment. The license is added afterwards by the be_license test,
# through portainer_licenses.
#
# Prints the variables the tests need; in GitHub Actions they also go to
# $GITHUB_ENV. Locally: eval "$(bash e2e-tests/lib/be-setup.sh)"

set -euo pipefail

CONTAINER="${BE_CONTAINER:-e2e-portainer-ee}"
PORT="${BE_PORT:-9443}"
VERSION="${PORTAINER_VERSION:-2.45.3}"
URL="https://localhost:${PORT}"
PASSWORD="e2e-Business-Admin-123"

fail() {
  echo "❌ $*" >&2
  exit 1
}

docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
echo "▶️ Start a fresh Portainer EE ${VERSION}" >&2
docker run -d --name "${CONTAINER}" -p "${PORT}:9443" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  "portainer/portainer-ee:${VERSION}" >/dev/null

UP=""
for _ in $(seq 1 60); do
  curl -sk -o /dev/null "${URL}/api/system/status" && UP=1 && break
  sleep 1
done
[ -n "${UP}" ] || fail "Portainer EE ${VERSION} did not come up on port ${PORT}"

TOKEN=""
for _ in $(seq 1 30); do
  TOKEN="$(docker logs "${CONTAINER}" 2>&1 | sed -E 's/\x1b\[[0-9;]*m//g' | grep -o -E 'setup_token=[0-9a-f]+' | head -1 | cut -d= -f2 || true)"
  [ -n "${TOKEN}" ] && break
  sleep 1
done

echo "▶️ Initialize the admin and create an API key" >&2
header=()
[ -n "${TOKEN}" ] && header=(-H "X-Setup-Token: ${TOKEN}")
curl -sk --fail-with-body -X POST "${URL}/api/users/admin/init" "${header[@]}" \
  -d "{\"Username\":\"admin\",\"Password\":\"${PASSWORD}\"}" >/dev/null ||
  fail "the admin could not be initialized"
JWT="$(curl -sk --fail-with-body -X POST "${URL}/api/auth" \
  -d "{\"Username\":\"admin\",\"Password\":\"${PASSWORD}\"}" | jq -r '.jwt')"
API_KEY="$(curl -sk --fail-with-body -X POST "${URL}/api/users/1/tokens" -H "Authorization: Bearer ${JWT}" \
  -d "{\"description\":\"e2e\",\"password\":\"${PASSWORD}\"}" | jq -r '.rawAPIKey')"
[ -n "${API_KEY}" ] && [ "${API_KEY}" != null ] || fail "no API key"
if [ -n "${GITHUB_ACTIONS:-}" ]; then
  echo "::add-mask::${API_KEY}"
fi

echo "▶️ Create the local Docker environment" >&2
ENDPOINT_ID="$(curl -sk --fail-with-body -X POST "${URL}/api/endpoints" -H "X-API-Key: ${API_KEY}" \
  -F Name=local -F EndpointCreationType=1 | jq -r '.Id')"

vars=(
  "PORTAINER_URL=${URL}"
  "PORTAINER_API_KEY=${API_KEY}"
  "TF_VAR_portainer_url=${URL}"
  "TF_VAR_portainer_api_key=${API_KEY}"
  "TF_VAR_endpoint_id=${ENDPOINT_ID}"
)
for v in "${vars[@]}"; do
  echo "export ${v}"
  if [ -n "${GITHUB_ENV:-}" ]; then
    echo "${v}" >>"${GITHUB_ENV}"
  fi
done
echo "✅ Portainer EE is ready on ${URL} (environment ${ENDPOINT_ID})" >&2
