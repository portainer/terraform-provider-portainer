#!/usr/bin/env bash
# Verify that every Terraform resource and data source served by the
# provider has a corresponding Markdown page under docs/.
#
# Source of truth = the type names the provider serves, NOT the Go file names.
# Some Go files use names that don't match the Terraform resource name (for
# example resource_endpoints_snapshot.go registers as portainer_endpoint_snapshot),
# so deriving the expected doc path from the file name produces false
# positives. The served type name is what users see on the Terraform
# Registry and is the stable contract.
#
# Resource:    portainer_<name>  -> docs/resources/<name>.md
# Data source: portainer_<name>  -> docs/data-sources/<name>.md
#
# Exits 0 when every entity has docs, otherwise prints the missing pages
# and exits 1.

set -euo pipefail
# A fixed locale keeps the awk regexes working the same everywhere: GNU awk
# matches nothing at all when the locale is unset.
export LC_ALL=C

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

# The provider reports its own type names (internal/tools/typenames), so
# nothing here depends on how the Go code registers them.
types="$(go run ./internal/tools/typenames)" || {
    echo "docs-presence-check: cannot list the provider's types" >&2
    exit 2
}
resources="$(awk '$1 == "resource" { sub(/^portainer_/, "", $2); print $2 }' <<< "$types")"
data_sources="$(awk '$1 == "data-source" { sub(/^portainer_/, "", $2); print $2 }' <<< "$types")"

if [[ -z "$resources" && -z "$data_sources" ]]; then
    echo "docs-presence-check: the provider reported no resources or data sources" >&2
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
