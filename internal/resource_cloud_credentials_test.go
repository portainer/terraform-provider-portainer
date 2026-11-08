package internal

import (
	"net/http"
	"testing"
)

// resource_cloud_credentials uses client.DoRequest:
//   - Create POSTs /cloud/credentials with JSON {provider, name, credentials}
//     and expects {"id": <int>} in the response.
//   - Read GETs /cloud/credentials/{id} and hydrates provider/name/credentials.
//   - Update PUTs /cloud/credentials/{id} with the same JSON body as Create.
//   - Delete DELETEs /cloud/credentials/{id}.

// TestCloudCredentialsCreate_HappyPath verifies the POST payload and that the
// numeric ID from the response is stringified into d.Id().
func TestCloudCredentialsCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/credentials", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 18,
	}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	_ = d.Set("cloud_provider", "aws")
	_ = d.Set("name", "ci-aws")
	_ = d.Set("credentials", map[string]interface{}{
		"accessKey": "AKIA...",
		"secretKey": "secret",
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "18" {
		t.Errorf("expected ID %q, got %q", "18", d.Id())
	}

	post := mock.FindRequest("POST", "/cloud/credentials")
	if post == nil {
		t.Fatal("expected POST /cloud/credentials to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["provider"]; got != "aws" {
		t.Errorf("payload.provider: expected %q, got %v", "aws", got)
	}
	if got := payload["name"]; got != "ci-aws" {
		t.Errorf("payload.name: expected %q, got %v", "ci-aws", got)
	}
	creds, ok := payload["credentials"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected credentials map, got %v", payload["credentials"])
	}
	if got := creds["accessKey"]; got != "AKIA..." {
		t.Errorf("credentials.accessKey: expected %q, got %v", "AKIA...", got)
	}
}

// TestCloudCredentialsRead_HappyPath verifies state is hydrated from the GET.
func TestCloudCredentialsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/credentials/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":       3,
		"name":     "prod-aws",
		"provider": "aws",
		"credentials": map[string]interface{}{
			"accessKey": "AKIA-PROD",
		},
	}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("3")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != "prod-aws" {
		t.Errorf("name: expected %q, got %v", "prod-aws", got)
	}
	if got := d.Get("cloud_provider"); got != "aws" {
		t.Errorf("cloud_provider: expected %q, got %v", "aws", got)
	}
	creds := d.Get("credentials").(map[string]interface{})
	if got := creds["accessKey"]; got != "AKIA-PROD" {
		t.Errorf("credentials.accessKey: expected %q, got %v", "AKIA-PROD", got)
	}
}

// TestCloudCredentialsUpdate_HappyPath confirms the PUT carries the new values
// in its JSON body and none of them in the request headers.
func TestCloudCredentialsUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/cloud/credentials/3", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("cloud_provider", "aws")
	_ = d.Set("name", "updated")
	_ = d.Set("credentials", map[string]interface{}{
		"accessKey": "AKIA-NEW",
	})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/cloud/credentials/3")
	if put == nil {
		t.Fatal("expected PUT /cloud/credentials/3 to be sent")
	}
	var payload CloudCredentialPayload
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if payload.Name != "updated" || payload.Provider != "aws" || payload.Credentials["accessKey"] != "AKIA-NEW" {
		t.Errorf("unexpected PUT body: %+v", payload)
	}
	if put.Headers.Get("Credentials") != "" {
		t.Error("credentials must not be sent as a request header")
	}
}

func TestCloudCredentialsRead_NotFoundDropsState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/cloud/credentials/3", RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("3")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the deleted credential dropped from state, got ID %q", d.Id())
	}
}

func TestCloudCredentialsDelete_AlreadyGone(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/cloud/credentials/3", RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("3")
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected Delete to succeed for a credential that is already gone, got %v", err)
	}
}

// TestCloudCredentialsDelete_HappyPath verifies the DELETE is issued.
func TestCloudCredentialsDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/cloud/credentials/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/cloud/credentials/5") == nil {
		t.Error("expected DELETE /cloud/credentials/5 to be sent")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after delete, got %q", d.Id())
	}
}

// TestCloudCredentialsCreate_HTTPError verifies error propagation.
func TestCloudCredentialsCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/credentials", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid provider"}`,
	))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	_ = d.Set("cloud_provider", "unknown")
	_ = d.Set("name", "x")
	_ = d.Set("credentials", map[string]interface{}{"k": "v"})

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestCloudCredentialsRead_KeepsMaskedSecrets is the regression test for a
// plan that changed the credentials on every run: Portainer returns secrets
// masked, and the mask went into state.
func TestCloudCredentialsRead_KeepsMaskedSecrets(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/cloud/credentials/1", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 1, "name": "aws", "provider": "aws",
		"credentials": map[string]interface{}{"accessKeyId": "AKIA2", "secretAccessKey": "******************"},
	}))

	r := resourceCloudCredentials()
	d := r.TestResourceData()
	d.SetId("1")
	_ = d.Set("credentials", map[string]interface{}{"accessKeyId": "AKIA1", "secretAccessKey": "real-secret"})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	creds := d.Get("credentials").(map[string]interface{})
	if creds["secretAccessKey"] != "real-secret" {
		t.Errorf("secretAccessKey: expected the configured value kept, got %v", creds["secretAccessKey"])
	}
	if creds["accessKeyId"] != "AKIA2" {
		t.Errorf("accessKeyId: a changed plain value must still show as drift, got %v", creds["accessKeyId"])
	}
}
