package internal

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// Annotations and labels of the Kubernetes objects managed through
// Portainer's own Kubernetes API (Gateway, Ingress) are shared with other
// writers: Gateway controllers, cert-manager, kubectl and Portainer itself add
// keys of their own. Like the kubernetes provider, only the keys the
// configuration sets are managed: Read keeps the other keys out of state
// unless they are configured, so they never read as drift, and an update
// sends them back unchanged instead of deleting them.

// k8sImportedKey marks, in private state, a state that comes straight from an
// import: there is no configuration yet to tell the managed keys apart, so
// the first Read takes every key the object has.
const k8sImportedKey = "imported"

// k8sMarkImported marks the state of an import (see k8sImportedKey).
func k8sMarkImported(ctx context.Context, private k8sfwPrivateWriter, diags *diag.Diagnostics) {
	diags.Append(private.SetKey(ctx, k8sImportedKey, []byte(`true`))...)
}

// k8sImported reports whether the prior state comes straight from an import.
func k8sImported(ctx context.Context, private k8sfwPrivateReader, diags *diag.Diagnostics) bool {
	v, d := private.GetKey(ctx, k8sImportedKey)
	diags.Append(d...)
	return len(v) > 0
}

// k8sClearImported removes the import mark once the state holds what was read.
func k8sClearImported(ctx context.Context, private k8sfwPrivateWriter, diags *diag.Diagnostics) {
	diags.Append(private.SetKey(ctx, k8sImportedKey, nil)...)
}

// k8sPortainerKey reports a key Portainer sets itself, such as the
// io.portainer.kubernetes.gateway.owner label.
func k8sPortainerKey(key string) bool {
	return strings.HasPrefix(key, "io.portainer.")
}

// k8sManagedMetadata returns the live keys the state should hold: the keys of
// managed (the prior state, or the plan after an apply) with their live
// values, a key gone from the object being left out so the plan puts it back.
// With all (right after an import) it is every live key but Portainer's own.
func k8sManagedMetadata(live map[string]string, managed map[string]interface{}, all bool) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range live {
		if _, ok := managed[k]; ok || (all && !k8sPortainerKey(k)) {
			out[k] = v
		}
	}
	return out
}

// k8sMergeForeignMetadata returns the keys an update sends: the planned ones,
// plus the live keys neither the prior state nor the plan holds, which belong
// to someone else and are sent back unchanged. A key in the prior state but no
// longer in the plan was removed from the configuration and is dropped.
func k8sMergeForeignMetadata(live map[string]string, prior, planned map[string]interface{}) map[string]string {
	out := make(map[string]string, len(live)+len(planned))
	for k, v := range live {
		_, wasManaged := prior[k]
		_, isManaged := planned[k]
		if !wasManaged && !isManaged {
			out[k] = v
		}
	}
	for k, v := range planned {
		s, _ := v.(string)
		out[k] = s
	}
	return out
}
