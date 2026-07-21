package internal

import (
	"net/http"
	"testing"
)

// TestBackupS3Cov3_Read_HTTPError exercises the >= 400 branch of
// resourceBackupS3Read: a non-2xx status on GET /backup/s3/settings must
// surface as an error.
func TestBackupS3Cov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/backup/s3/settings", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceBackupS3()
	d := r.TestResourceData()
	d.SetId("portainer_backup_s3")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestBackupS3Cov3_Read_DecodeError exercises the JSON decode-failure branch:
// a 200 response with a body that is not valid JSON must surface as an error.
func TestBackupS3Cov3_Read_DecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/backup/s3/settings", RespondString(
		http.StatusOK, "application/json",
		`not-json`,
	))

	r := resourceBackupS3()
	d := r.TestResourceData()
	d.SetId("portainer_backup_s3")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on invalid JSON body, got nil")
	}
}
