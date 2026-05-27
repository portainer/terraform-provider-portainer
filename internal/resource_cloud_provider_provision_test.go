package internal

import (
	"net/http"
	"testing"
)

// resource_cloud_provider_provision is an action-style resource:
//   - Create POSTs JSON to /cloud/{provider}/provision and reads the
//     {"Id": <int>} response. The resource ID is the stringified Id.
//   - Read is schema.Noop — once provisioning is fired, the resource records
//     only the ID; there is no follow-up sync.
//   - Delete is schema.RemoveFromState — there is no "deprovision" endpoint
//     and the state is simply dropped on destroy.
//
// Not covered (would require a much longer test):
//   - The context-deadline / 30-minute timeout path. The httptest server
//     responds synchronously so the timeout has no effect in unit tests.
//   - Provider-specific payload shapes (the resource treats `payload` as an
//     untyped map and forwards it through json.Marshal verbatim).

// TestCloudProvisionCreate_HappyPath_Civo exercises a typical provision flow:
// payload is a flat map of strings; response is {"Id": int}.
func TestCloudProvisionCreate_HappyPath_Civo(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/civo/provision", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 27,
	}))

	r := resourcePortainerCloudProvision()
	d := r.TestResourceData()
	_ = d.Set("cloud_provider", "civo")
	_ = d.Set("payload", map[string]interface{}{
		"name":              "ci-cluster",
		"region":            "FRA1",
		"nodeCount":         "3",
		"nodeSize":          "g3.small",
		"kubernetesVersion": "1.29",
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "27" {
		t.Errorf("expected ID %q, got %q", "27", d.Id())
	}

	post := mock.FindRequest("POST", "/cloud/civo/provision")
	if post == nil {
		t.Fatal("expected POST /cloud/civo/provision to be sent")
	}
	if got := post.Headers.Get("X-API-Key"); got != "test-api-key" {
		t.Errorf("X-API-Key header: expected %q, got %q", "test-api-key", got)
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["name"]; got != "ci-cluster" {
		t.Errorf("payload.name: expected %q, got %v", "ci-cluster", got)
	}
	if got := payload["region"]; got != "FRA1" {
		t.Errorf("payload.region: expected %q, got %v", "FRA1", got)
	}
}

// TestCloudProvisionCreate_RoutesByProvider verifies the URL path is derived
// from `cloud_provider`.
func TestCloudProvisionCreate_RoutesByProvider(t *testing.T) {
	for _, prov := range []string{"digitalocean", "linode", "amazon", "azure", "gke"} {
		t.Run(prov, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/cloud/"+prov+"/provision", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id": 1,
			}))

			r := resourcePortainerCloudProvision()
			d := r.TestResourceData()
			_ = d.Set("cloud_provider", prov)
			_ = d.Set("payload", map[string]interface{}{
				"name": "x",
			})

			if err := r.Create(d, mock.Client()); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			if mock.FindRequest("POST", "/cloud/"+prov+"/provision") == nil {
				t.Errorf("expected POST /cloud/%s/provision to be sent", prov)
			}
		})
	}
}

// TestCloudProvisionCreate_HTTPError verifies a 4xx response surfaces as a
// terraform-level error and the ID stays empty.
func TestCloudProvisionCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/cloud/civo/provision", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid region"}`,
	))

	r := resourcePortainerCloudProvision()
	d := r.TestResourceData()
	_ = d.Set("cloud_provider", "civo")
	_ = d.Set("payload", map[string]interface{}{
		"name": "broken",
	})

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestCloudProvisionCreate_NoAuth verifies the explicit error returned when
// the client has neither an API key nor a JWT token.
func TestCloudProvisionCreate_NoAuth(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	r := resourcePortainerCloudProvision()
	d := r.TestResourceData()
	_ = d.Set("cloud_provider", "civo")
	_ = d.Set("payload", map[string]interface{}{
		"name": "x",
	})

	err := r.Create(d, client)
	if err == nil {
		t.Fatal("expected an authentication error, got nil")
	}
}
