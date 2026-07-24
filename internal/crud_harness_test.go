package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// This file provides a small table-driven harness that collapses the repetitive
// setup shared by the hand-written resource_*_cov*_test.go files: standing up a
// MockServer, registering a batch of routes, building a ResourceData with its
// fields, invoking one CRUD handler, and asserting the id/error outcome.
//
// It is intentionally single-operation oriented (create OR read OR update OR
// delete per case), because that is how the existing coverage tests are shaped
// and because the MockServer matches one handler per method+path. A test that
// needs different responses for the same path across phases should still be
// written by hand.

// apiRoute is one mock HTTP route answered with a status code and body. Body is
// marshaled as JSON unless it is a string, which is written verbatim (useful for
// error-body assertions). Status defaults to 200 when zero.
type apiRoute struct {
	Method string
	Path   string
	Status int
	Body   interface{}
}

// registerRoutes wires a batch of apiRoutes into the mock in one call.
func registerRoutes(m *MockServer, routes ...apiRoute) {
	for _, rt := range routes {
		status := rt.Status
		if status == 0 {
			status = http.StatusOK
		}
		if s, ok := rt.Body.(string); ok {
			m.On(rt.Method, rt.Path, RespondString(status, "application/json", s))
		} else {
			m.On(rt.Method, rt.Path, RespondJSON(status, rt.Body))
		}
	}
}

// newTestData builds a ResourceData for r, applies the given schema fields, and
// sets id when non-empty. It fails the test on a schema type mismatch instead of
// silently dropping the value, which is the class of bug the hand-written
// `_ = d.Set(...)` idiom hides.
func newTestData(t *testing.T, r *schema.Resource, id string, fields map[string]interface{}) *schema.ResourceData {
	t.Helper()
	d := r.TestResourceData()
	for k, v := range fields {
		if err := d.Set(k, v); err != nil {
			t.Fatalf("set %q: %v", k, err)
		}
	}
	if id != "" {
		d.SetId(id)
	}
	return d
}

// crudCase declaratively drives a single CRUD operation against a fresh mock.
//
//	Op     – "create" | "read" | "update" | "delete" (default "create")
//	ID     – seeds d.SetId before the op (required for read/update/delete)
//	Fields – schema values to set on the ResourceData
//	Routes – mock routes registered before the op runs
//	WantErr       – expect the handler to return an error
//	WantErrSubstr – if set, the returned error must contain this substring
//	WantID        – expected d.Id() after the op (e.g. cleared to "" on 404)
//	Check         – optional extra assertions on the resulting ResourceData
type crudCase struct {
	Op            string
	Resource      *schema.Resource
	ID            string
	Fields        map[string]interface{}
	Routes        []apiRoute
	WantErr       bool
	WantErrSubstr string
	WantID        string
	Check         func(t *testing.T, d *schema.ResourceData)
}

func (c crudCase) run(t *testing.T) (*schema.ResourceData, *MockServer) {
	t.Helper()
	m := NewMockServer(t)
	registerRoutes(m, c.Routes...)
	d := newTestData(t, c.Resource, c.ID, c.Fields)

	var err error
	switch c.Op {
	case "", "create":
		err = rcCreate(c.Resource, d, m.Client())
	case "read":
		err = rcRead(c.Resource, d, m.Client())
	case "update":
		err = rcUpdate(c.Resource, d, m.Client())
	case "delete":
		err = rcDelete(c.Resource, d, m.Client())
	default:
		t.Fatalf("unknown crudCase.Op %q", c.Op)
	}

	if c.WantErr && err == nil {
		t.Fatalf("%s: expected error, got nil", c.Op)
	}
	if !c.WantErr && err != nil {
		t.Fatalf("%s: unexpected error: %v", c.Op, err)
	}
	if c.WantErrSubstr != "" && (err == nil || !strings.Contains(err.Error(), c.WantErrSubstr)) {
		t.Fatalf("%s: error %q does not contain %q", c.Op, err, c.WantErrSubstr)
	}
	if c.WantID != "" && d.Id() != c.WantID {
		t.Errorf("%s: id = %q, want %q", c.Op, d.Id(), c.WantID)
	}
	if c.Check != nil {
		c.Check(t, d)
	}
	return d, m
}
