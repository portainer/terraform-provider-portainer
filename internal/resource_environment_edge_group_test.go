package internal

import (
	"net/http"
	"testing"
)

// TestEnvironmentUpdate_EdgeAgentSendsGroupAndName verifies that moving an
// Edge Agent environment to another group, or renaming it, reaches Portainer,
// while the URL and TLS fields stay out of the request.
func TestEnvironmentUpdate_EdgeAgentSendsGroupAndName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   21,
		"Name": "edge-renamed",
		"Type": 4,
	}))
	mock.On("GET", "/endpoints/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      21,
		"Name":    "edge-renamed",
		"Type":    4,
		"GroupId": 5,
		"URL":     "portainer.example.com",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("21")
	_ = d.Set("name", "edge-renamed")
	_ = d.Set("environment_address", "https://portainer.example.com")
	_ = d.Set("type", 4)
	_ = d.Set("group_id", 5)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/21")
	if put == nil {
		t.Fatal("expected PUT /endpoints/21")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if payload["groupID"] != float64(5) {
		t.Errorf("groupID: expected 5, got %v", payload["groupID"])
	}
	if payload["name"] != "edge-renamed" {
		t.Errorf("name: expected %q, got %v", "edge-renamed", payload["name"])
	}
	for _, key := range []string{"url", "tls", "tlsskipVerify", "tlsskipClientVerify"} {
		if v, ok := payload[key]; ok && v != false && v != "" {
			t.Errorf("%s must not be sent for an Edge Agent environment, got %v", key, v)
		}
	}
}
