package internal

import (
	"net/http"
	"testing"
)

// TestBackupS3Create_HappyPath verifies that resourceBackupS3 POSTs the S3
// credentials and bucket configuration to /backup/s3/execute. The endpoint
// returns 204 No Content on success, and the resource sets a static ID.
func TestBackupS3Create_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/backup/s3/execute", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r := resourceBackupS3()
	d := r.TestResourceData()
	_ = d.Set("access_key_id", "AKIAEXAMPLE")
	_ = d.Set("secret_access_key", "supersecret")
	_ = d.Set("bucket_name", "portainer-backups")
	_ = d.Set("region", "eu-central-1")
	_ = d.Set("s3_compatible_host", "https://s3.amazonaws.com")
	_ = d.Set("password", "topsecret")
	_ = d.Set("cron_rule", "0 3 * * *")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "portainer_backup_s3" {
		t.Errorf("expected static ID %q, got %q", "portainer_backup_s3", d.Id())
	}

	req := mock.FindRequest("POST", "/backup/s3/execute")
	if req == nil {
		t.Fatal("expected POST /backup/s3/execute")
	}

	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	// Verify camelCase field names (the Portainer API contract).
	checks := map[string]string{
		"accessKeyID":      "AKIAEXAMPLE",
		"secretAccessKey":  "supersecret",
		"bucketName":       "portainer-backups",
		"region":           "eu-central-1",
		"s3CompatibleHost": "https://s3.amazonaws.com",
		"password":         "topsecret",
		"cronRule":         "0 3 * * *",
	}
	for k, want := range checks {
		if got, _ := payload[k].(string); got != want {
			t.Errorf("payload[%q]: expected %q, got %v", k, want, payload[k])
		}
	}
}

// TestBackupS3Create_OmitsOptionalCronRule verifies that an unset cron_rule
// is NOT included in the request payload (the resource uses GetOk for that field).
func TestBackupS3Create_OmitsOptionalCronRule(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/backup/s3/execute", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r := resourceBackupS3()
	d := r.TestResourceData()
	_ = d.Set("access_key_id", "AKIAEXAMPLE")
	_ = d.Set("secret_access_key", "supersecret")
	_ = d.Set("bucket_name", "portainer-backups")
	_ = d.Set("region", "eu-central-1")
	_ = d.Set("s3_compatible_host", "https://s3.amazonaws.com")
	_ = d.Set("password", "topsecret")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("POST", "/backup/s3/execute")
	if req == nil {
		t.Fatal("expected POST /backup/s3/execute")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, present := payload["cronRule"]; present {
		t.Errorf("expected cronRule to be omitted when not set, got %v", payload["cronRule"])
	}
}

// TestBackupS3Create_HTTPError verifies that a non-204 response surfaces as
// an error and leaves the ID empty.
func TestBackupS3Create_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/backup/s3/execute", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"s3 upload failed"}`,
	))

	r := resourceBackupS3()
	d := r.TestResourceData()
	_ = d.Set("access_key_id", "x")
	_ = d.Set("secret_access_key", "y")
	_ = d.Set("bucket_name", "b")
	_ = d.Set("region", "r")
	_ = d.Set("s3_compatible_host", "h")
	_ = d.Set("password", "p")

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestBackupS3Read_PopulatesState verifies that Read decodes the
// /backup/s3/settings response into resource state.
func TestBackupS3Read_PopulatesState(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/backup/s3/settings", RespondJSON(http.StatusOK, map[string]interface{}{
		"accessKeyID":      "AKIAREAD",
		"secretAccessKey":  "secret-read",
		"bucketName":       "read-bucket",
		"region":           "us-east-1",
		"s3CompatibleHost": "https://s3.us-east-1.amazonaws.com",
		"password":         "pw-read",
		"cronRule":         "*/15 * * * *",
	}))

	r := resourceBackupS3()
	d := r.TestResourceData()
	d.SetId("portainer_backup_s3")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("bucket_name"); got != "read-bucket" {
		t.Errorf("bucket_name: expected %q, got %v", "read-bucket", got)
	}
	if got := d.Get("region"); got != "us-east-1" {
		t.Errorf("region: expected %q, got %v", "us-east-1", got)
	}
	if got := d.Get("cron_rule"); got != "*/15 * * * *" {
		t.Errorf("cron_rule: expected %q, got %v", "*/15 * * * *", got)
	}
	if d.Id() != "portainer_backup_s3" {
		t.Errorf("expected stable ID, got %q", d.Id())
	}
}

// TestBackupS3Delete_ClearsID verifies that Delete simply removes the
// resource from state (the operation is irreversible on the API side).
func TestBackupS3Delete_ClearsID(t *testing.T) {
	r := resourceBackupS3()
	d := r.TestResourceData()
	d.SetId("portainer_backup_s3")

	if err := rcDelete(r, d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after Delete, got %q", d.Id())
	}
}
