package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceUserActivityRead_Activity verifies that the activity log
// endpoint is queried, results are populated, and total_count is set.
func TestDataSourceUserActivityRead_Activity(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/logs", RespondJSON(http.StatusOK, map[string]interface{}{
		"logs": []map[string]interface{}{
			{
				"id":        1,
				"timestamp": 1700000000,
				"username":  "alice",
				"action":    "login",
				"context":   "endpoint1",
			},
			{
				"id":        2,
				"timestamp": 1700001000,
				"username":  "bob",
				"action":    "logout",
				"context":   "endpoint2",
			},
		},
		"totalCount": 2,
	}))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "activity")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	logs, ok := d.Get("activity_logs").([]interface{})
	if !ok {
		t.Fatalf("expected activity_logs list, got %T", d.Get("activity_logs"))
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 activity logs, got %d", len(logs))
	}
	if got := d.Get("total_count"); got != 2 {
		t.Errorf("total_count: expected 2, got %v", got)
	}
	if d.Id() == "" {
		t.Error("expected synthetic ID to be set, got empty")
	}

	// Verify the request used the activity-logs path. Note: defaults are
	// only applied when state is hydrated from a config — TestResourceData()
	// starts empty so GetOk treats unset ints as zero and skips them.
	req := mock.FindRequest("GET", "/useractivity/logs")
	if req == nil {
		t.Fatal("expected GET /useractivity/logs to be sent")
	}
}

// TestDataSourceUserActivityRead_Auth verifies the auth-logs branch decodes
// a flat array.
func TestDataSourceUserActivityRead_Auth(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/authlogs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"id":        1,
			"timestamp": 1700000000,
			"username":  "alice",
			"type":      1,
			"origin":    "192.168.1.1",
			"context":   1,
		},
	}))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "auth")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	logs, ok := d.Get("auth_logs").([]interface{})
	if !ok {
		t.Fatalf("expected auth_logs list, got %T", d.Get("auth_logs"))
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 auth log, got %d", len(logs))
	}
	first := logs[0].(map[string]interface{})
	if first["username"] != "alice" {
		t.Errorf("first.username: expected alice, got %v", first["username"])
	}
}

// TestDataSourceUserActivityRead_InvalidLogType verifies that an unknown
// log_type is rejected before any HTTP call is made.
func TestDataSourceUserActivityRead_InvalidLogType(t *testing.T) {
	mock := NewMockServer(t)

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "bogus")

	err := ds.Read(d, mock.Client())
	if err == nil {
		t.Fatal("expected error for invalid log_type, got nil")
	}
}

// TestDataSourceUserActivityRead_HTTPError verifies HTTP error propagation.
func TestDataSourceUserActivityRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/useractivity/logs", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceUserActivity()
	d := ds.TestResourceData()
	_ = d.Set("log_type", "activity")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
