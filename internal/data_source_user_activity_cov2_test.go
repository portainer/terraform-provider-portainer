package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestUserActivityCov2_Read_AllFilters exercises every query-parameter building
// branch of the activity read: offset, limit, before, after, sort_by,
// sort_desc, keyword, and the activity-only username/context list filters.
func TestUserActivityCov2_Read_AllFilters(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/logs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs":       []map[string]interface{}{},
		"totalCount": 0,
	}))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "activity")
	_ = d.Set("offset", 5)
	_ = d.Set("limit", 10)
	_ = d.Set("before", 100)
	_ = d.Set("after", 50)
	_ = d.Set("sort_by", "Timestamp")
	_ = d.Set("sort_desc", true)
	_ = d.Set("keyword", "foo")
	_ = d.Set("username", []interface{}{"alice", "bob"})
	_ = d.Set("context", []interface{}{"ctx1"})

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/useractivity/logs")
	if req == nil {
		t.Fatal("expected GET /useractivity/logs")
	}
	q := req.Query
	for _, want := range []string{
		"offset=5", "limit=10", "before=100", "after=50",
		"sortBy=Timestamp", "sortDesc=true", "keyword=foo",
		"username=", "context=",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q missing %q", q, want)
		}
	}
}

// TestUserActivityCov2_Read_ActivityDecodeError covers the decode-error branch
// of the activity log response.
func TestUserActivityCov2_Read_ActivityDecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/logs", RespondString(
		http.StatusOK, "application/json", `{ not valid json`,
	))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "activity")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed activity JSON, got nil")
	}
}

// TestUserActivityCov2_Read_AuthDecodeError covers the decode-error branch of
// the auth log response.
func TestUserActivityCov2_Read_AuthDecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/authlogs", RespondString(
		http.StatusOK, "application/json", `{ not an array`,
	))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "auth")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed auth JSON, got nil")
	}
}

// TestUserActivityCov2_Read_AuthHTTPError covers the HTTP-error branch on the
// auth-logs path (the base suite only exercises it for activity logs).
func TestUserActivityCov2_Read_AuthHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/authlogs", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "auth")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
