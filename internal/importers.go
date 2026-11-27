package internal

import (
	"fmt"
	"strconv"
	"strings"
)

// Importers for resources whose ID is composed of several attributes. Each
// one only splits the ID into the attributes Read needs; Terraform runs Read
// right after, which fills in the rest or reports the object as missing.

// importIDParts splits id on sep into exactly n parts, naming the expected
// format in the error.
func importIDParts(id, sep string, n int, format string) ([]string, error) {
	parts := strings.SplitN(id, sep, n)
	if len(parts) != n {
		return nil, fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("invalid import ID %q: expected %s", id, format)
		}
	}
	return parts, nil
}

// importIDPartsCompat splits id in the provider-wide format, parts separated
// by ":". An ID without ":" is split on legacySep instead, the separator the
// resource used before v3.0.0, so existing import blocks and scripts keep
// working; the importer then stores the ID in the current format.
func importIDPartsCompat(id, legacySep string, n int, format string) ([]string, error) {
	sep := ":"
	if !strings.Contains(id, ":") {
		sep = legacySep
	}
	return importIDParts(id, sep, n, format)
}

// splitEndpointImportID splits "<endpoint_id>:<object>", or the
// "<endpoint_id>-<object>" form from before v3.0.0, for a Docker object that
// lives on one environment.
func splitEndpointImportID(id, format string) (int, string, error) {
	parts, err := importIDPartsCompat(id, "-", 2, format)
	if err != nil {
		return 0, "", err
	}
	endpointID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	return endpointID, parts[1], nil
}
