package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerImageRead_ExactTag finds an image when the requested
// name includes a tag and matches a RepoTag exactly.
func TestDataSourceDockerImageRead_ExactTag(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/4/docker/images/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "sha256:img-a", "RepoTags": []string{"alpine:3.18", "alpine:latest"}},
		{"Id": "sha256:img-b", "RepoTags": []string{"nginx:1.25"}},
	}))

	ds := dataSourceDockerImage()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("name", "nginx:1.25")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "sha256:img-b" {
		t.Errorf("expected ID %q, got %q", "sha256:img-b", d.Id())
	}
}

// TestDataSourceDockerImageRead_ImpliedLatest verifies that a name without a
// colon is matched against ":latest" automatically.
func TestDataSourceDockerImageRead_ImpliedLatest(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/4/docker/images/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "sha256:img-a", "RepoTags": []string{"alpine:3.18", "alpine:latest"}},
	}))

	ds := dataSourceDockerImage()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("name", "alpine")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "sha256:img-a" {
		t.Errorf("expected ID %q, got %q", "sha256:img-a", d.Id())
	}
}

// TestDataSourceDockerImageRead_NotFound errors when no RepoTag matches.
func TestDataSourceDockerImageRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/4/docker/images/json", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "sha256:img-a", "RepoTags": []string{"alpine:3.18"}},
	}))

	ds := dataSourceDockerImage()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("name", "nginx:1.25")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker image, got nil")
	}
}

// TestDataSourceDockerImageRead_HTTPError propagates HTTP errors.
func TestDataSourceDockerImageRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/4/docker/images/json", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceDockerImage()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("name", "nginx:1.25")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
