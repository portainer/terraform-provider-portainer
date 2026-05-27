package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceHelmGitDryRunRead_HappyPath verifies the data source POSTs to
// /endpoints/{id}/kubernetes/helm/git/dryrun, stores the rendered manifest,
// and constructs a deterministic ID.
func TestDataSourceHelmGitDryRunRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	rendered := "apiVersion: v1\nkind: Service\nmetadata:\n  name: app\n"
	mock.On("POST", "/endpoints/3/kubernetes/helm/git/dryrun", RespondJSON(http.StatusOK, map[string]interface{}{
		"manifest":  rendered,
		"name":      "my-release",
		"namespace": "default",
		"version":   2,
	}))

	ds := dataSourceHelmGitDryRun()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("repository_url", "https://github.com/owner/chart.git")
	_ = d.Set("reference_name", "refs/heads/main")
	_ = d.Set("chart_path", "charts/myapp")
	_ = d.Set("values_files", []interface{}{"values.yaml", "values-prod.yaml"})
	_ = d.Set("namespace", "default")
	_ = d.Set("release_name", "my-release")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Id(); got != "helm-git-dryrun-3-https://github.com/owner/chart.git" {
		t.Errorf("unexpected ID: %q", got)
	}
	if got := d.Get("manifest"); got != rendered {
		t.Errorf("manifest mismatch: %v", got)
	}
	if got := d.Get("release_version"); got != 2 {
		t.Errorf("release_version: expected 2, got %v", got)
	}

	// Verify the payload uses the API's expected keys.
	post := mock.FindRequest("POST", "/endpoints/3/kubernetes/helm/git/dryrun")
	if post == nil {
		t.Fatal("expected POST to dryrun endpoint")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["repositoryURL"] != "https://github.com/owner/chart.git" {
		t.Errorf("payload.repositoryURL: got %v", payload["repositoryURL"])
	}
	if payload["repositoryReferenceName"] != "refs/heads/main" {
		t.Errorf("payload.repositoryReferenceName: got %v", payload["repositoryReferenceName"])
	}
	if payload["helmChartPath"] != "charts/myapp" {
		t.Errorf("payload.helmChartPath: got %v", payload["helmChartPath"])
	}
	if payload["name"] != "my-release" {
		t.Errorf("payload.name: got %v", payload["name"])
	}
	files, ok := payload["helmValuesFiles"].([]interface{})
	if !ok || len(files) != 2 || files[0] != "values.yaml" || files[1] != "values-prod.yaml" {
		t.Errorf("payload.helmValuesFiles: got %v", payload["helmValuesFiles"])
	}
}

// TestDataSourceHelmGitDryRunRead_HTTPError verifies non-2xx is surfaced.
func TestDataSourceHelmGitDryRunRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/helm/git/dryrun", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"chart render failed"}`,
	))

	ds := dataSourceHelmGitDryRun()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("repository_url", "https://example.com/chart.git")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
