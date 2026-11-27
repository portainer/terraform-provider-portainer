package internal

import (
	"fmt"
	"sort"
)

// Drift of portainer_kubernetes_ingresses. Portainer writes the Ingress from
// its flat model (K8sIngressInfo) but has no GET for one, so Read takes the
// networking.k8s.io/v1 object from the Kubernetes proxy and maps it back the
// way Portainer's convertToK8sIngress built it. Like k8sReadManifestDrift for
// the manifest resources, only what the configuration sets is compared, and
// only after normalising what the server stores differently:
//   - paths: Portainer groups them into one rule per host by iterating a Go
//     map, so the rule order is random; they are compared as a multiset;
//   - hosts: every host gets a rule, so a host with paths is also listed
//     among the rule hosts; only hosts missing from the rules, or rule hosts
//     neither listed nor used by a path, are drift;
//   - class_name and annotations are compared only when configured: a
//     cluster's default IngressClass is filled in by an admission plugin, and
//     controllers add annotations of their own (k8sManagedMetadata);
//   - labels are never compared: Portainer ignores them and labels the
//     Ingress with its own io.portainer.kubernetes.ingress.owner only.
// When something differs, the state takes the live value so the plan shows
// the drift and the next apply puts the configuration back. Right after an
// import every argument but labels is taken from the Ingress.

// k8sIngressObject is the part of a networking.k8s.io/v1 Ingress Read uses.
type k8sIngressObject struct {
	Metadata struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		IngressClassName *string `json:"ingressClassName"`
		TLS              []struct {
			Hosts      []string `json:"hosts"`
			SecretName string   `json:"secretName"`
		} `json:"tls"`
		Rules []struct {
			Host string `json:"host"`
			HTTP *struct {
				Paths []struct {
					Path     string  `json:"path"`
					PathType *string `json:"pathType"`
					Backend  struct {
						Service *struct {
							Name string `json:"name"`
							Port struct {
								Number int `json:"number"`
							} `json:"port"`
						} `json:"service"`
					} `json:"backend"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
	} `json:"spec"`
}

// refreshIngressFromLive stores the drift of the live Ingress in data, the
// SDKv2 view of the prior state.
func refreshIngressFromLive(data map[string]interface{}, live *k8sIngressObject, imported bool) {
	liveClass := ""
	if live.Spec.IngressClassName != nil {
		liveClass = *live.Spec.IngressClassName
	}
	if class, _ := data["class_name"].(string); imported || class != "" {
		data["class_name"] = liveClass
	}

	if annotations := k8sfwMap(data, "annotations"); imported || len(annotations) > 0 {
		data["annotations"] = k8sManagedMetadata(live.Metadata.Annotations, annotations, imported)
	}

	livePaths := ingressLivePaths(live)
	priorPaths := k8sfwList(data, "paths")
	if imported || !ingressSamePaths(priorPaths, livePaths) {
		data["paths"] = ingressMergePaths(priorPaths, livePaths)
	}

	liveTLS := make([]interface{}, 0, len(live.Spec.TLS))
	for _, t := range live.Spec.TLS {
		liveTLS = append(liveTLS, map[string]interface{}{
			"hosts":       ingressStringList(t.Hosts),
			"secret_name": t.SecretName,
		})
	}
	if imported || !ingressSameTLS(k8sfwList(data, "tls"), liveTLS) {
		data["tls"] = liveTLS
	}

	if hosts, drifted := ingressHostsDrift(k8sfwStrings(data["hosts"]), priorPaths, live, imported); drifted {
		data["hosts"] = hosts
	}
}

// ingressLivePaths flattens the rules of the Ingress into "paths" blocks.
func ingressLivePaths(live *k8sIngressObject) []interface{} {
	out := []interface{}{}
	for _, rule := range live.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, p := range rule.HTTP.Paths {
			pathType, service, port := "", "", 0
			if p.PathType != nil {
				pathType = *p.PathType
			}
			if p.Backend.Service != nil {
				service, port = p.Backend.Service.Name, p.Backend.Service.Port.Number
			}
			out = append(out, map[string]interface{}{
				"host":         rule.Host,
				"path":         p.Path,
				"path_type":    pathType,
				"port":         port,
				"service_name": service,
			})
		}
	}
	return out
}

// ingressPathKey identifies a "paths" block by all of its fields.
func ingressPathKey(raw interface{}) string {
	m, _ := raw.(map[string]interface{})
	return fmt.Sprintf("%v\x00%v\x00%v\x00%v\x00%v", m["host"], m["path"], m["path_type"], m["port"], m["service_name"])
}

// ingressSamePaths compares the "paths" blocks in any order.
func ingressSamePaths(prior, live []interface{}) bool {
	if len(prior) != len(live) {
		return false
	}
	count := map[string]int{}
	for _, p := range prior {
		count[ingressPathKey(p)]++
	}
	for _, p := range live {
		key := ingressPathKey(p)
		if count[key] == 0 {
			return false
		}
		count[key]--
	}
	return true
}

// ingressMergePaths returns the live paths, those still in the prior state
// first and in its order, so the plan shows only what changed.
func ingressMergePaths(prior, live []interface{}) []interface{} {
	remaining := map[string][]interface{}{}
	for _, p := range live {
		key := ingressPathKey(p)
		remaining[key] = append(remaining[key], p)
	}
	out := make([]interface{}, 0, len(live))
	for _, p := range prior {
		key := ingressPathKey(p)
		if l := remaining[key]; len(l) > 0 {
			out = append(out, l[0])
			remaining[key] = l[1:]
		}
	}
	for _, p := range live {
		key := ingressPathKey(p)
		if l := remaining[key]; len(l) > 0 {
			out = append(out, l[0])
			remaining[key] = l[1:]
		}
	}
	return out
}

// ingressSameTLS compares the "tls" blocks in order: Portainer keeps it.
func ingressSameTLS(prior, live []interface{}) bool {
	if len(prior) != len(live) {
		return false
	}
	for i := range prior {
		p, _ := prior[i].(map[string]interface{})
		l, _ := live[i].(map[string]interface{})
		ph, lh := k8sfwStrings(p["hosts"]), k8sfwStrings(l["hosts"])
		ps, _ := p["secret_name"].(string)
		ls, _ := l["secret_name"].(string)
		if ps != ls || len(ph) != len(lh) {
			return false
		}
		for j := range ph {
			if ph[j] != lh[j] {
				return false
			}
		}
	}
	return true
}

// ingressHostsDrift compares "hosts" with the hosts of the Ingress rules. It
// returns the hosts to store and whether they differ from prior: the prior
// hosts still having a rule, in order, then the rule hosts neither listed
// nor used by a path, sorted. After an import it is every rule host.
func ingressHostsDrift(prior []string, priorPaths []interface{}, live *k8sIngressObject, imported bool) ([]interface{}, bool) {
	liveHosts := map[string]bool{}
	for _, rule := range live.Spec.Rules {
		liveHosts[rule.Host] = true
	}
	if imported {
		hosts := make([]string, 0, len(liveHosts))
		for h := range liveHosts {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		return ingressStringList(hosts), true
	}

	expected := map[string]bool{}
	for _, p := range priorPaths {
		m, _ := p.(map[string]interface{})
		host, _ := m["host"].(string)
		expected[host] = true
	}
	drifted := false
	out := []interface{}{}
	for _, h := range prior {
		expected[h] = true
		if liveHosts[h] {
			out = append(out, h)
		} else {
			drifted = true
		}
	}
	var extra []string
	for h := range liveHosts {
		if !expected[h] {
			extra = append(extra, h)
		}
	}
	sort.Strings(extra)
	for _, h := range extra {
		out = append(out, h)
	}
	return out, drifted || len(extra) > 0
}

func ingressStringList(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, v := range values {
		out = append(out, v)
	}
	return out
}
