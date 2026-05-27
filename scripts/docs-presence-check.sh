#!/usr/bin/env bash
# Verify that every Terraform resource and data source registered in
# internal/provider.go has a corresponding Markdown page under docs/.
#
# Source of truth = the resource map in provider.go, NOT the Go file names.
# Some Go files use names that don't match the Terraform resource name (for
# example resource_endpoints_snapshot.go registers as portainer_endpoint_snapshot),
# so deriving the expected doc path from the file name produces false
# positives. The provider.go registration is what users see on the
# Terraform Registry and is the stable contract.
#
# Resource:    portainer_<name>  -> docs/resources/<name>.md
# Data source: portainer_<name>  -> docs/data-sources/<name>.md
#
# Exits 0 when every entity has docs, otherwise prints the missing pages
# and exits 1.

set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

provider_file="internal/provider.go"
if [[ ! -f "$provider_file" ]]; then
    echo "docs-presence-check: cannot find $provider_file" >&2
    exit 2
fi

# Extract resource and data-source registrations from the ResourcesMap and
# DataSourcesMap blocks. Both blocks use the same syntax:
#   "portainer_<name>": resourceXxx(),
#
# We rely on awk to track which block we are inside, then pull the quoted
# name. This is robust to formatting differences as long as the maps start
# with "ResourcesMap:" / "DataSourcesMap:" and use map[string]*schema.*{}.

resources="$(awk '
    /ResourcesMap:/        { in_res = 1; in_ds = 0; next }
    /DataSourcesMap:/      { in_res = 0; in_ds = 1; next }
    /^\s*\},?\s*$/         { in_res = 0; in_ds = 0; next }
    in_res && /"portainer_/ {
        if (match($0, /"portainer_[A-Za-z0-9_]+"/)) {
            name = substr($0, RSTART + 11, RLENGTH - 12)
            print name
        }
    }
' "$provider_file")"

data_sources="$(awk '
    /ResourcesMap:/        { in_res = 1; in_ds = 0; next }
    /DataSourcesMap:/      { in_res = 0; in_ds = 1; next }
    /^\s*\},?\s*$/         { in_res = 0; in_ds = 0; next }
    in_ds && /"portainer_/ {
        if (match($0, /"portainer_[A-Za-z0-9_]+"/)) {
            name = substr($0, RSTART + 11, RLENGTH - 12)
            print name
        }
    }
' "$provider_file")"

if [[ -z "$resources" && -z "$data_sources" ]]; then
    echo "docs-presence-check: no resources or data sources extracted from $provider_file" >&2
    echo "(the awk parser may be out of sync with provider.go formatting)" >&2
    exit 2
fi

missing=()

while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    if [[ ! -f "docs/resources/${name}.md" ]]; then
        missing+=("docs/resources/${name}.md")
    fi
done <<< "$resources"

while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    if [[ ! -f "docs/data-sources/${name}.md" ]]; then
        missing+=("docs/data-sources/${name}.md")
    fi
done <<< "$data_sources"

if [[ ${#missing[@]} -eq 0 ]]; then
    echo "docs-presence-check: OK — every resource and data source has a docs page."
    exit 0
fi

echo "docs-presence-check: ${#missing[@]} docs page(s) missing:"
for m in "${missing[@]}"; do
    echo "  $m"
done
exit 1
