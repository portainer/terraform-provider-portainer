package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Test helpers of the batch A2 Kubernetes resources.

// a2LastBody decodes the body of the last request with the method and path.
func a2LastBody(t *testing.T, mock *MockServer, method, path string, v interface{}) {
	t.Helper()
	reqs := mock.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Method == method && reqs[i].Path == path {
			if err := reqs[i].DecodeJSON(v); err != nil {
				t.Fatalf("decode %s %s: %v", method, path, err)
			}
			return
		}
	}
	t.Fatalf("no %s %s request was sent", method, path)
}

// a2Count counts the requests with the method and path.
func a2Count(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// a2Check wraps a Go assertion as a TestCheckFunc.
func a2Check(f func() error) resource.TestCheckFunc {
	return func(*terraform.State) error { return f() }
}

// a2Upgrade applies each configuration with v3.0.0, then plans it with this
// build against the same fake API: the plan has to be empty. Each
// configuration is a test case of its own, destroyed before the next.
func a2Upgrade(t *testing.T, mock *MockServer, configs ...string) {
	t.Helper()
	for _, c := range configs {
		config := mockProviderConfig(mock) + c
		resource.Test(t, resource.TestCase{
			IsUnitTest: true,
			Steps: []resource.TestStep{
				{ExternalProviders: previousRelease, Config: config},
				{
					ProtoV5ProviderFactories: testProtoV5Factories,
					Config:                   config,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{a2ExplainPlan{t}, plancheck.ExpectEmptyPlan()},
					},
				},
			},
		})
	}
}

// a2Store is a JSON document store a fake API serves.
type a2Store struct {
	mu   sync.Mutex
	docs map[string]interface{}
}

func newA2Store() *a2Store { return &a2Store{docs: map[string]interface{}{}} }

func (s *a2Store) put(key string, v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(v)
	var doc interface{}
	_ = json.Unmarshal(b, &doc)
	s.docs[key] = doc
}

func (s *a2Store) get(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.docs[key]
	return v, ok
}

func (s *a2Store) del(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, key)
}

// serve answers GET path with the document at key, 404 when there is none.
func (s *a2Store) serve(mock *MockServer, path, key string) {
	mock.On("GET", path, func(w http.ResponseWriter, r *http.Request) {
		if v, ok := s.get(key); ok {
			RespondJSON(http.StatusOK, v)(w, r)
			return
		}
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
	})
}

// a2Decode decodes a request body a fake handler received.
func a2Decode(r *http.Request, v interface{}) {
	_ = json.NewDecoder(r.Body).Decode(v)
}

// a2ExplainPlan logs the before and after values of every planned change, so
// a failed empty-plan check says what changed.
type a2ExplainPlan struct{ t *testing.T }

func (c a2ExplainPlan) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, _ *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Change == nil || rc.Change.Actions.NoOp() {
			continue
		}
		before, _ := json.Marshal(rc.Change.Before)
		after, _ := json.Marshal(rc.Change.After)
		c.t.Logf("%s %v\nbefore: %s\nafter:  %s", rc.Address, rc.Change.Actions, before, after)
	}
}
