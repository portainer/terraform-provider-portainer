package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubernetesPodLogs_HappyPath verifies the plain-text log body is
// stored verbatim and that the optional parameters reach the API.
func TestDataSourceKubernetesPodLogs_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	logs := "2026-09-01T10:00:00Z starting\n2026-09-01T10:00:01Z ready\n"
	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log",
		RespondString(http.StatusOK, "text/plain", logs))

	ds := dataSourceKubernetesPodLogs()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 6)
	_ = d.Set("namespace", "default")
	_ = d.Set("pod_name", "web-1")
	_ = d.Set("container", "web")
	_ = d.Set("tail_lines", 200)
	_ = d.Set("since_seconds", 3600)
	_ = d.Set("timestamps", true)
	_ = d.Set("previous", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("logs"); got != logs {
		t.Errorf("logs mismatch: %q", got)
	}
	if got := d.Get("line_count"); got != 2 {
		t.Errorf("line_count: expected 2 non-empty lines, got %v", got)
	}
	if got := d.Id(); got != "6/default/web-1/log" {
		t.Errorf("id: got %q", got)
	}

	get := mock.FindRequest("GET", "/kubernetes/6/namespaces/default/pods/web-1/log")
	if get == nil {
		t.Fatal("expected GET to the pod log endpoint")
	}
	for _, want := range []string{"container=web", "tailLines=200", "sinceSeconds=3600", "timestamps=true", "previous=true"} {
		if !strings.Contains(get.Query, want) {
			t.Errorf("query %q should contain %q", get.Query, want)
		}
	}
	// follow would hold the response open until the client disconnects, which
	// must never happen during a plan or apply.
	if strings.Contains(get.Query, "follow") {
		t.Errorf("query must never request follow, got %q", get.Query)
	}
}

// TestDataSourceKubernetesPodLogs_OmitsUnsetOptions verifies that unset options
// are left out entirely, so Kubernetes applies its own defaults.
func TestDataSourceKubernetesPodLogs_OmitsUnsetOptions(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log",
		RespondString(http.StatusOK, "text/plain", ""))

	ds := dataSourceKubernetesPodLogs()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 6)
	_ = d.Set("namespace", "default")
	_ = d.Set("pod_name", "web-1")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("line_count"); got != 0 {
		t.Errorf("line_count: expected 0 for an empty log, got %v", got)
	}

	get := mock.FindRequest("GET", "/kubernetes/6/namespaces/default/pods/web-1/log")
	if get.Query != "" {
		t.Errorf("expected no query parameters when nothing is configured, got %q", get.Query)
	}
}

// TestDataSourceKubernetesPodLogs_ErrorStatus verifies a non-2xx response is
// reported as an error rather than stored as if it were log output. The
// endpoint answers text/plain, so this path does not go through doJSON.
func TestDataSourceKubernetesPodLogs_ErrorStatus(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/6/namespaces/default/pods/web-1/log",
		RespondString(http.StatusForbidden, "application/json", `{"message":"Unauthorized access to the Kubernetes API"}`))

	ds := dataSourceKubernetesPodLogs()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 6)
	_ = d.Set("namespace", "default")
	_ = d.Set("pod_name", "web-1")

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected Read to fail on a 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should carry the status code, got: %v", err)
	}
	if got := d.Get("logs"); got != "" {
		t.Errorf("logs should stay empty on failure, got %q", got)
	}
}
