package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"testing"
)

// fakeAlertingRules is a stateful stand-in for /observability/alerting/rules
// and the groups and tiers endpoints of a rule. Rules are kept as the raw
// JSON objects Portainer would return, so a PUT replaces a rule with exactly
// what the provider sent.
type fakeAlertingRules struct {
	mu     sync.Mutex
	nextID int
	rules  map[int]map[string]interface{}
	// postWithoutID makes POST answer with an empty object, as the spec allows.
	postWithoutID bool
	// renderExpr makes PUT rebuild promqlExpr from the condition operator and
	// threshold, as Portainer does for a built-in rule.
	renderExpr bool
}

func newFakeAlertingRules(seed ...map[string]interface{}) *fakeAlertingRules {
	f := &fakeAlertingRules{nextID: 100, rules: map[int]map[string]interface{}{}}
	for _, rule := range seed {
		f.rules[int(toFloat(rule["id"]))] = normalizeJSON(rule)
	}
	return f
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case float64:
		return n
	}
	return 0
}

// normalizeJSON round-trips v through JSON, as the wire would.
func normalizeJSON(v interface{}) map[string]interface{} {
	raw, _ := json.Marshal(v)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func (f *fakeAlertingRules) get(id int) map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rules[id]
}

func (f *fakeAlertingRules) remove(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rules, id)
}

func (f *fakeAlertingRules) register(m *MockServer) {
	m.On("GET", "/observability/alerting/rules", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		ids := make([]int, 0, len(f.rules))
		for id := range f.rules {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		out := make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			out = append(out, f.rules[id])
		}
		RespondJSON(http.StatusOK, out)(w, nil)
	})
	m.On("POST", "/observability/alerting/rules", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AlertingRule map[string]interface{} `json:"AlertingRule"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		rule := body.AlertingRule
		rule["id"] = float64(f.nextID)
		f.rules[f.nextID] = rule
		if f.postWithoutID {
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			return
		}
		RespondJSON(http.StatusOK, rule)(w, r)
	})
	for id := 1; id <= 120; id++ {
		id := id
		p := fmt.Sprintf("/observability/alerting/rules/%d", id)
		m.On("GET", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			rule, ok := f.rules[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"alert rule not found"}`)(w, nil)
				return
			}
			RespondJSON(http.StatusOK, rule)(w, nil)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				AlertingRule map[string]interface{} `json:"AlertingRule"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.rules[id]; !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"alert rule not found"}`)(w, r)
				return
			}
			if f.renderExpr {
				body.AlertingRule["promqlExpr"] = fmt.Sprintf("increase(auth_failures[5m]) %v %v", body.AlertingRule["conditionOperator"], body.AlertingRule["threshold"])
			}
			f.rules[id] = body.AlertingRule
			RespondJSON(http.StatusOK, body.AlertingRule)(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.rules, id)
			w.WriteHeader(http.StatusNoContent)
		})
		m.On("PUT", p+"/groups", func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			rule, ok := f.rules[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"alert rule not found"}`)(w, r)
				return
			}
			rule["endpointGroupIds"] = body["endpointGroupIds"]
			RespondJSON(http.StatusOK, map[string]interface{}{"id": id})(w, r)
		})
		m.On("PUT", p+"/tiers", func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			rule, ok := f.rules[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"alert rule not found"}`)(w, r)
				return
			}
			rule["enabled"] = body["enabled"]
			rule["tiers"] = body["tiers"]
			rule["useTiers"] = true
			for _, k := range []string{"description", "conditionOperator", "duration"} {
				if v, ok := body[k]; ok {
					rule[k] = v
				}
			}
			RespondJSON(http.StatusOK, map[string]interface{}{"id": id})(w, r)
		})
	}
}

// fakeAlertingSettings stands in for /observability/alerting/settings: a
// list of entries, a PUT upserting the entry its body names (or the first).
type fakeAlertingSettings struct {
	mu      sync.Mutex
	entries []map[string]interface{}
	// noIDInResponse makes the PUT answer without an id.
	noIDInResponse bool
}

func (f *fakeAlertingSettings) register(m *MockServer) {
	m.On("GET", "/observability/alerting/settings", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := f.entries
		if out == nil {
			out = []map[string]interface{}{}
		}
		RespondJSON(http.StatusOK, out)(w, nil)
	})
	m.On("PUT", "/observability/alerting/settings", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AlertingSettings map[string]interface{} `json:"AlertingSettings"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		entry := body.AlertingSettings
		id := toFloat(entry["id"])
		if id == 0 {
			id = 1
			if len(f.entries) > 0 {
				id = toFloat(f.entries[0]["id"])
			}
		}
		entry["id"] = id
		if _, ok := entry["status"]; !ok {
			entry["status"] = "connected"
		}
		replaced := false
		for i, e := range f.entries {
			if toFloat(e["id"]) == id {
				f.entries[i] = entry
				replaced = true
			}
		}
		if !replaced {
			f.entries = append(f.entries, entry)
		}
		if f.noIDInResponse {
			RespondJSON(http.StatusOK, map[string]interface{}{"enabled": entry["enabled"]})(w, r)
			return
		}
		RespondJSON(http.StatusOK, entry)(w, r)
	})
}

// requireAlertingBE skips an acceptance test of the Business Edition
// alerting API on a CE instance.
func requireAlertingBE(t *testing.T) {
	t.Helper()
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("alerting is a Portainer Business Edition feature; set PORTAINER_ACC_BE=1 to run against a BE instance")
	}
}

// alertingEnvOrSkip returns the environment variable name, skipping the test
// without it.
func alertingEnvOrSkip(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skip(name + " must be set for this acceptance test")
	}
	return v
}
