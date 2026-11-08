package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

// k8sConfirmExistsByGET issues a GET against a Kubernetes object to verify it
// still exists. On HTTP 404 it clears the resource ID via d.SetId("") so the
// next plan recreates it (out-of-band deletion detection). On any other non-2xx
// it returns diagnostics carrying the response body. Resources defined by a
// manifest use k8sReadManifestDrift, which also detects a changed content.
//
// Returns nil diagnostics with the ID preserved when the object exists, or nil
// diagnostics with the ID cleared when it is gone. Callers should check d.Id() == ""
// before setting any other fields.
func k8sConfirmExistsByGET(ctx context.Context, d *schema.ResourceData, client *APIClient, url, kind string) diag.Diagnostics {
	if err := doJSON(ctx, client, http.MethodGet, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read %s: %w", kind, err))
	}
	return nil
}

// k8sReadManifestDrift is k8sConfirmExistsByGET that also detects a change of
// the object's content. The live object is always a superset of the authored
// manifest - the API server adds status, metadata and defaulted fields - so
// only the fields the manifest sets are compared. When one of them differs,
// manifest in state becomes those fields with their live values, which shows
// exactly the drifted fields in the plan; the next apply puts the authored
// manifest back. Without drift the state is left alone, so the authored text
// (comments, formatting) stays as written.
func k8sReadManifestDrift(ctx context.Context, d *schema.ResourceData, client *APIClient, url, kind string) diag.Diagnostics {
	var live map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &live); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read %s: %w", kind, err))
	}

	// After an import there is no manifest to compare with yet.
	authored, err := parseManifest(d.Get("manifest").(string))
	if err != nil || len(authored) == 0 {
		return nil
	}
	authored = k8sComparable(k8sNormalizeAuthored(authored), live)
	if k8sSubsetEqual(authored, live) {
		return nil
	}

	var drifted bytes.Buffer
	enc := yaml.NewEncoder(&drifted)
	enc.SetIndent(2)
	if err := enc.Encode(k8sProjectLive(authored, live)); err != nil {
		return diag.FromErr(fmt.Errorf("failed to render the live %s: %w", kind, err))
	}
	if err := d.Set("manifest", drifted.String()); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// k8sScaledKinds are workloads whose replica count is commonly changed by
// something else - a HorizontalPodAutoscaler or
// portainer_kubernetes_deployment_scale - so spec.replicas is not compared:
// putting it back on every apply would fight them and recreate the workload.
var k8sScaledKinds = map[string]bool{"Deployment": true, "StatefulSet": true, "ReplicaSet": true}

// k8sComparable drops the authored fields the API server is known to discard
// or that other tools manage, so they never read as drift.
func k8sComparable(authored, live map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(authored))
	for k, v := range authored {
		out[k] = v
	}
	// A namespace on a cluster-scoped object (ClusterRole, StorageClass,
	// PersistentVolume, ...) is dropped by the server.
	if meta, ok := out["metadata"].(map[string]interface{}); ok {
		liveMeta, _ := live["metadata"].(map[string]interface{})
		if _, hasNS := liveMeta["namespace"]; !hasNS {
			if _, ok := meta["namespace"]; ok {
				copied := make(map[string]interface{}, len(meta))
				for k, v := range meta {
					copied[k] = v
				}
				delete(copied, "namespace")
				out["metadata"] = copied
			}
		}
	}
	if kind, _ := out["kind"].(string); k8sScaledKinds[kind] {
		if spec, ok := out["spec"].(map[string]interface{}); ok {
			if _, ok := spec["replicas"]; ok {
				copied := make(map[string]interface{}, len(spec))
				for k, v := range spec {
					copied[k] = v
				}
				delete(copied, "replicas")
				out["spec"] = copied
			}
		}
	}
	return out
}

// k8sNormalizeAuthored rewrites what the API server stores differently from
// how it is written: a Secret's stringData comes back base64-encoded in data.
func k8sNormalizeAuthored(authored map[string]interface{}) map[string]interface{} {
	stringData, ok := authored["stringData"].(map[string]interface{})
	if authored["kind"] != "Secret" || !ok {
		return authored
	}
	out := make(map[string]interface{}, len(authored))
	for k, v := range authored {
		out[k] = v
	}
	data := map[string]interface{}{}
	if existing, ok := authored["data"].(map[string]interface{}); ok {
		for k, v := range existing {
			data[k] = v
		}
	}
	for k, v := range stringData {
		data[k] = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(v)))
	}
	delete(out, "stringData")
	out["data"] = data
	return out
}

// k8sSubsetEqual reports whether every field set in authored has the same
// value in live. Lists are compared element by element and must have the
// same length; maps only by the authored keys.
func k8sSubsetEqual(authored, live interface{}) bool {
	switch a := authored.(type) {
	case nil:
		return true
	case map[string]interface{}:
		l, _ := live.(map[string]interface{})
		for k, v := range a {
			lv, ok := l[k]
			if !ok {
				if k8sIsEmpty(v) {
					continue
				}
				return false
			}
			if !k8sSubsetEqual(v, lv) {
				return false
			}
		}
		return true
	case []interface{}:
		l, _ := live.([]interface{})
		if len(a) != len(l) {
			return false
		}
		for i := range a {
			if !k8sSubsetEqual(a[i], l[i]) {
				return false
			}
		}
		return true
	default:
		return k8sScalarEqual(authored, live)
	}
}

// k8sScalarEqual compares two scalars the way the API server treats them:
// numbers whatever their Go type, and resource quantities by value
// ("0.5" and "500m", "1024Mi" and "1Gi").
func k8sScalarEqual(a, b interface{}) bool {
	if fmt.Sprint(a) == fmt.Sprint(b) {
		return true
	}
	if _, isBool := a.(bool); isBool {
		return false
	}
	av, aok := k8sQuantity(a)
	bv, bok := k8sQuantity(b)
	if !aok || !bok {
		return false
	}
	return math.Abs(av-bv) <= 1e-9*math.Max(math.Abs(av), math.Abs(bv))
}

var k8sQuantityPattern = regexp.MustCompile(`^([+-]?[0-9]*\.?[0-9]+(?:[eE][+-]?[0-9]+)?)(m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei)?$`)

var k8sQuantitySuffixes = map[string]float64{
	"": 1, "m": 1e-3, "k": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18,
	"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50, "Ei": 1 << 60,
}

func k8sQuantity(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float64:
		return n, true
	case string:
		m := k8sQuantityPattern.FindStringSubmatch(n)
		if m == nil {
			return 0, false
		}
		f, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, false
		}
		return f * k8sQuantitySuffixes[m[2]], true
	}
	return 0, false
}

func k8sIsEmpty(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]interface{}:
		return len(x) == 0
	case []interface{}:
		return len(x) == 0
	case string:
		return x == ""
	}
	return false
}

// k8sProjectLive returns live restricted to the fields authored sets, for a
// plan that shows only what drifted.
func k8sProjectLive(authored, live interface{}) interface{} {
	switch a := authored.(type) {
	case map[string]interface{}:
		l, ok := live.(map[string]interface{})
		if !ok {
			return live
		}
		out := map[string]interface{}{}
		for k, v := range a {
			if lv, ok := l[k]; ok {
				out[k] = k8sProjectLive(v, lv)
			}
		}
		return out
	case []interface{}:
		l, ok := live.([]interface{})
		if !ok {
			return live
		}
		out := make([]interface{}, len(l))
		for i := range l {
			if i < len(a) {
				out[i] = k8sProjectLive(a[i], l[i])
			} else {
				out[i] = l[i]
			}
		}
		return out
	default:
		return live
	}
}

// k8sUpdateManifest applies a changed manifest to the existing object in
// place, with a JSON merge patch, so that a Deployment rolls out and a Service
// keeps its cluster IP instead of being deleted and created again. The patch
// is the new manifest plus a null for every field the previous one set and the
// new one does not, so a field removed from the manifest is removed from the
// object too.
//
// name is the object's name from the resource ID; identity lists the
// attributes that address the object (namespace, type). The object is deleted
// and created again when it cannot be changed in place: another name, kind or
// address, or a field the API server does not allow to change (HTTP 422, e.g.
// a Job's pod template or a Deployment's selector).
func k8sUpdateManifest(ctx context.Context, d *schema.ResourceData, client *APIClient, url, name, kind string, del, create func() diag.Diagnostics, identity ...string) diag.Diagnostics {
	recreate := func() diag.Diagnostics {
		if diags := del(); diags.HasError() {
			return diags
		}
		if err := k8sWaitDeleted(ctx, client, url); err != nil {
			return diag.FromErr(fmt.Errorf("%s: %w", kind, err))
		}
		return create()
	}

	oldRaw, newRaw := d.GetChange("manifest")
	desired, err := parseManifest(newRaw.(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("manifest must be valid JSON or YAML: %w", err))
	}
	// Empty after an import, which leaves manifest unset.
	previous, _ := parseManifest(oldRaw.(string))

	if !k8sSameObject(previous, desired, name) || d.HasChanges(identity...) {
		return recreate()
	}

	patch := k8sMergePatch(previous, desired)
	err = doJSONWithHeaders(ctx, client, http.MethodPatch, url, patch, nil,
		map[string]string{"Content-Type": "application/merge-patch+json"})
	var se *apiStatusError
	if errors.As(err, &se) && se.StatusCode == http.StatusUnprocessableEntity {
		log.Printf("[INFO] %s cannot be updated in place, recreating it: %s", kind, se.Body)
		return recreate()
	}
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update %s: %w", kind, err))
	}
	return nil
}

// k8sSameObject reports whether desired still describes the object called
// name: same metadata.name, and the same kind and apiVersion as before when
// the previous manifest says.
func k8sSameObject(previous, desired map[string]interface{}, name string) bool {
	meta, _ := desired["metadata"].(map[string]interface{})
	if newName, _ := meta["name"].(string); newName != name {
		return false
	}
	for _, key := range []string{"kind", "apiVersion"} {
		if before, ok := previous[key]; ok && desired[key] != nil && fmt.Sprint(before) != fmt.Sprint(desired[key]) {
			return false
		}
	}
	return true
}

// k8sMergePatch builds the JSON merge patch (RFC 7386) that turns an object
// created from previous into one matching desired. Both are normalized first,
// so a key dropped from a Secret's stringData is removed from its data. Lists
// are replaced as a whole, as merge patch does, and the server applies its
// defaults to the new elements again.
//
// spec.replicas of a scaled workload is only sent when the manifest changes
// it, so an in-place update does not undo an autoscaler or
// portainer_kubernetes_deployment_scale; see k8sScaledKinds.
func k8sMergePatch(previous, desired map[string]interface{}) map[string]interface{} {
	previous = k8sNormalizeAuthored(previous)
	desired = k8sNormalizeAuthored(desired)
	patch := k8sPatchMap(previous, desired)

	if kind, _ := desired["kind"].(string); k8sScaledKinds[kind] {
		spec, _ := patch["spec"].(map[string]interface{})
		prevSpec, _ := previous["spec"].(map[string]interface{})
		before, had := prevSpec["replicas"]
		if after, ok := spec["replicas"]; ok && (!had || k8sScalarEqual(before, after)) {
			delete(spec, "replicas")
		}
	}
	return patch
}

func k8sPatchMap(previous, desired map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(desired))
	for k, v := range desired {
		prevMap, prevIsMap := previous[k].(map[string]interface{})
		if newMap, ok := v.(map[string]interface{}); ok && prevIsMap {
			out[k] = k8sPatchMap(prevMap, newMap)
			continue
		}
		out[k] = v
	}
	for k := range previous {
		if _, kept := desired[k]; !kept {
			out[k] = nil
		}
	}
	return out
}

// k8sDeletePollInterval is how often k8sWaitDeleted checks; tests shorten it.
var k8sDeletePollInterval = time.Second

// k8sWaitDeleted waits until the object at url is gone. A deleted object can
// linger while its finalizers run, and creating it again meanwhile fails with
// "object is being deleted".
func k8sWaitDeleted(ctx context.Context, client *APIClient, url string) error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err := doJSON(ctx, client, http.MethodGet, url, nil, nil)
		if isAPINotFound(err) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the previous object was still being deleted after 2 minutes")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(k8sDeletePollInterval):
		}
	}
}
