package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEdgeJobRead_HappyPath verifies list+filter on /edge_jobs by Name.
func TestDataSourceEdgeJobRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 2, "Name": "other", "CronExpression": "0 * * * *"},
		{"Id": 11, "Name": "nightly", "CronExpression": "0 3 * * *"},
	}))

	ds := dataSourceEdgeJob()
	d := ds.TestResourceData()
	_ = d.Set("name", "nightly")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}
	if got := d.Get("cron_expression"); got != "0 3 * * *" {
		t.Errorf("cron_expression: expected %q, got %v", "0 3 * * *", got)
	}
}

// TestDataSourceEdgeJobRead_NotFound verifies a missing name returns an error.
func TestDataSourceEdgeJobRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceEdgeJob()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when edge job not found, got nil")
	}
}

// TestDataSourceEdgeJobRead_HTTPError verifies non-200 status is surfaced.
func TestDataSourceEdgeJobRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs", RespondString(http.StatusInternalServerError,
		"application/json", `{"message":"boom"}`))

	ds := dataSourceEdgeJob()
	d := ds.TestResourceData()
	_ = d.Set("name", "x")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
