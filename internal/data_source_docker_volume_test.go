package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerVolumeRead_HappyPath matches a volume by Name within
// the {"Volumes":[...]} envelope and exposes driver/mountpoint.
func TestDataSourceDockerVolumeRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/volumes", RespondJSON(http.StatusOK, map[string]interface{}{
		"Volumes": []map[string]interface{}{
			{"Name": "other", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/other/_data"},
			{"Name": "data", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/data/_data"},
		},
	}))

	ds := dataSourceDockerVolume()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("name", "data")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "data" {
		t.Errorf("expected ID %q (volume name), got %q", "data", d.Id())
	}
	if got := d.Get("driver"); got != "local" {
		t.Errorf("driver: expected %q, got %v", "local", got)
	}
	if got := d.Get("mount_point"); got != "/var/lib/docker/volumes/data/_data" {
		t.Errorf("mount_point: expected data mountpoint, got %v", got)
	}
}

// TestDataSourceDockerVolumeRead_NotFound errors out if no volume matches.
func TestDataSourceDockerVolumeRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/volumes", RespondJSON(http.StatusOK, map[string]interface{}{
		"Volumes": []map[string]interface{}{
			{"Name": "other", "Driver": "local", "Mountpoint": "/x"},
		},
	}))

	ds := dataSourceDockerVolume()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker volume, got nil")
	}
}

// TestDataSourceDockerVolumeRead_HTTPError propagates HTTP errors.
func TestDataSourceDockerVolumeRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/volumes", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceDockerVolume()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("name", "data")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
