#!/usr/bin/env bash
# Verify that every Terraform resource and data source registered in
# internal/provider.go is demonstrated by at least one example under
# examples/.
#
# The check is CONTENT-based, not directory-name based: an example "exists"
# for portainer_<name> when some examples/**/*.tf file declares the matching
# block:
#
#   resource "portainer_<name>" ...      (for resources)
#   data     "portainer_<name>" ...      (for data sources)
#
# This tolerates the repository's real layout, where examples are sometimes
# grouped (examples/deployment/ covers portainer_deploy + portainer_check,
# examples/registry/ covers portainer_registry + portainer_registry_access)
# or directory-named loosely. The block declaration is the source of truth.
#
# Exits 0 when every entity is demonstrated, otherwise prints the gaps and
# exits 1.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

provider_file="internal/provider.go"
if [[ ! -f "$provider_file" ]]; then
    echo "examples-presence-check: cannot find $provider_file" >&2
    exit 2
fi

resources="$(awk '
    /ResourcesMap:/   { in_res = 1; in_ds = 0; next }
    /DataSourcesMap:/ { in_res = 0; in_ds = 1; next }
    /^\s*\},?\s*$/    { in_res = 0; in_ds = 0; next }
    in_res && /"portainer_/ {
        if (match($0, /"portainer_[A-Za-z0-9_]+"/)) print substr($0, RSTART + 11, RLENGTH - 12)
    }
' "$provider_file")"

data_sources="$(awk '
    /ResourcesMap:/   { in_res = 1; in_ds = 0; next }
    /DataSourcesMap:/ { in_res = 0; in_ds = 1; next }
    /^\s*\},?\s*$/    { in_res = 0; in_ds = 0; next }
    in_ds && /"portainer_/ {
        if (match($0, /"portainer_[A-Za-z0-9_]+"/)) print substr($0, RSTART + 11, RLENGTH - 12)
    }
' "$provider_file")"

if [[ -z "$resources" && -z "$data_sources" ]]; then
    echo "examples-presence-check: no entities parsed from $provider_file" >&2
    exit 2
fi

# Collect every example .tf file once.
mapfile -t tf_files < <(find examples -type f -name '*.tf' 2>/dev/null)
if [[ ${#tf_files[@]} -eq 0 ]]; then
    echo "examples-presence-check: no .tf files found under examples/" >&2
    exit 2
fi

# declared <kind> <name> — true if a "<kind> \"portainer_<name>\"" block exists.
declared() {
    local kind="$1" name="$2"
    # Exact quoted match avoids edge_configuration matching edge_configurations.
    grep -qE "^[[:space:]]*${kind}[[:space:]]+\"portainer_${name}\"" "${tf_files[@]}"
}

missing=()

while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    declared "resource" "$name" || missing+=("resource portainer_$name")
done <<< "$resources"

while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    declared "data" "$name" || missing+=("data source portainer_$name")
done <<< "$data_sources"

if [[ ${#missing[@]} -eq 0 ]]; then
    echo "examples-presence-check: OK — every resource and data source is demonstrated in examples/."
    exit 0
fi

echo "examples-presence-check: ${#missing[@]} entit(ies) with no example block:"
for m in "${missing[@]}"; do
    echo "  $m"
done
exit 1
