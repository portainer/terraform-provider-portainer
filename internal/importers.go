package internal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

// setImportInts parses each value as an int and stores it under its key.
func setImportInts(d *schema.ResourceData, id, format string, values map[string]string) error {
	for key, raw := range values {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("invalid import ID %q: expected %s", id, format)
		}
		if err := d.Set(key, v); err != nil {
			return err
		}
	}
	return nil
}

// setImportPrincipal stores "team/<id>" or "user/<id>" as team_id or user_id.
func setImportPrincipal(d *schema.ResourceData, id, format, kind, raw string) error {
	switch kind {
	case "team":
		return setImportInts(d, id, format, map[string]string{"team_id": raw})
	case "user":
		return setImportInts(d, id, format, map[string]string{"user_id": raw})
	}
	return fmt.Errorf("invalid import ID %q: expected %s", id, format)
}

func importRegistryAccess(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<registry_id>:<endpoint_id>:team:<team_id> or <registry_id>:<endpoint_id>:user:<user_id>"
	parts, err := importIDPartsCompat(d.Id(), "/", 4, format)
	if err != nil {
		return nil, err
	}
	if err := setImportInts(d, d.Id(), format, map[string]string{"registry_id": parts[0], "endpoint_id": parts[1]}); err != nil {
		return nil, err
	}
	if err := setImportPrincipal(d, d.Id(), format, parts[2], parts[3]); err != nil {
		return nil, err
	}
	d.SetId(strings.Join(parts, ":"))
	return []*schema.ResourceData{d}, nil
}

func importEndpointGroupAccess(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<endpoint_group_id>:team:<team_id> or <endpoint_group_id>:user:<user_id>"
	parts, err := importIDPartsCompat(d.Id(), "/", 3, format)
	if err != nil {
		return nil, err
	}
	if err := setImportInts(d, d.Id(), format, map[string]string{"endpoint_group_id": parts[0]}); err != nil {
		return nil, err
	}
	if err := setImportPrincipal(d, d.Id(), format, parts[1], parts[2]); err != nil {
		return nil, err
	}
	d.SetId(strings.Join(parts, ":"))
	return []*schema.ResourceData{d}, nil
}

func importDockerNode(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<endpoint_id>:<node_id>"
	parts, err := importIDPartsCompat(d.Id(), "-", 2, format)
	if err != nil {
		return nil, err
	}
	if err := setImportInts(d, d.Id(), format, map[string]string{"endpoint_id": parts[0]}); err != nil {
		return nil, err
	}
	if err := d.Set("node_id", parts[1]); err != nil {
		return nil, err
	}
	d.SetId(strings.Join(parts, ":"))
	return []*schema.ResourceData{d}, nil
}

func importKubernetesPersistentVolume(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<environment_id>:<name>"
	parts, err := importIDPartsCompat(d.Id(), "/", 2, format)
	if err != nil {
		return nil, err
	}
	if err := setImportInts(d, d.Id(), format, map[string]string{"environment_id": parts[0]}); err != nil {
		return nil, err
	}
	if err := d.Set("name", parts[1]); err != nil {
		return nil, err
	}
	d.SetId(strings.Join(parts, ":"))
	return []*schema.ResourceData{d}, nil
}

func importKubernetesIngressControllers(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	if err := setImportInts(d, d.Id(), "<environment_id>", map[string]string{"environment_id": d.Id()}); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

// importStack takes "<endpoint_id>:<stack_id>:<deployment_type>[:<method>]";
// the resource ID is the stack ID alone. The parts used to be separated by
// "-", which is still accepted.
func importStack(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<endpoint_id>:<stack_id>:<deployment_type>[:<method>]"
	id := d.Id()
	sep := ":"
	if !strings.Contains(id, ":") {
		sep = "-"
	}
	parts := strings.Split(id, sep)
	if len(parts) < 3 || len(parts) > 4 {
		return nil, fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("invalid import ID %q: expected %s", id, format)
		}
	}
	if err := setImportInts(d, id, format, map[string]string{"endpoint_id": parts[0]}); err != nil {
		return nil, err
	}
	stackID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid import ID %q: expected %s", id, format)
	}
	if err := d.Set("deployment_type", parts[2]); err != nil {
		return nil, err
	}
	if len(parts) == 4 {
		if err := d.Set("method", parts[3]); err != nil {
			return nil, err
		}
	}
	d.SetId(strconv.Itoa(stackID))
	return []*schema.ResourceData{d}, nil
}

// importSingleton imports a resource that manages instance-wide settings and
// always uses the same ID, whatever ID was given on the command line.
func importSingleton(id string) schema.StateContextFunc {
	return func(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
		d.SetId(id)
		return []*schema.ResourceData{d}, nil
	}
}
