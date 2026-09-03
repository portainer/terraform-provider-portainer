package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// gitopsSourceDataWithChanges builds a ResourceData carrying a real diff, so
// the d.HasChanges gates in Update are exercised. changed maps attribute names
// to their new value; the old value is left empty, which is enough to make
// HasChanges report true.
func gitopsSourceDataWithChanges(t *testing.T, id string, changed map[string]string) *schema.ResourceData {
	t.Helper()
	r := resourceGitopsSource()
	attrs := map[string]string{"id": id, "url": "https://github.com/acme/old.git", "name": "infra"}
	diffAttrs := map[string]*terraform.ResourceAttrDiff{}
	for k, v := range changed {
		diffAttrs[k] = &terraform.ResourceAttrDiff{Old: attrs[k], New: v}
	}
	d, err := schema.InternalMap(r.Schema).Data(&terraform.InstanceState{ID: id, Attributes: attrs}, &terraform.InstanceDiff{Attributes: diffAttrs})
	if err != nil {
		t.Fatalf("failed to build diffed ResourceData: %v", err)
	}
	return d
}

func gitopsSourceDetailBody(id int) map[string]interface{} {
	return map[string]interface{}{
		"id": id, "name": "infra", "url": "https://github.com/acme/infra.git",
		"type": "git", "status": "healthy", "interval": "5m", "lastSync": 1756900000,
		"connection": map[string]interface{}{
			"tlsSkipVerify":  true,
			"authentication": map[string]interface{}{"username": "ci"},
		},
		"access": map[string]interface{}{
			"public": false, "teams": []int{4}, "users": []int{7, 9},
		},
	}
}

// TestGitopsSourceCreate_HappyPath verifies the create payload uses the API's
// camelCase keys and that the follow-up read populates state from SourceDetail.
func TestGitopsSourceCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/gitops/sources/git", RespondJSON(http.StatusCreated, map[string]interface{}{"id": 12}))
	mock.On("GET", "/gitops/sources/12", RespondJSON(http.StatusOK, gitopsSourceDetailBody(12)))

	r := resourceGitopsSource()
	d := r.TestResourceData()
	_ = d.Set("url", "https://github.com/acme/infra.git")
	_ = d.Set("name", "infra")
	_ = d.Set("interval", "5m")
	_ = d.Set("username", "ci")
	_ = d.Set("password", "token")
	_ = d.Set("tls_skip_verify", true)
	_ = d.Set("administrators_only", true)
	_ = d.Set("user_accesses", []interface{}{7, 9})
	_ = d.Set("team_accesses", []interface{}{4})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "12" {
		t.Errorf("id: expected %q, got %q", "12", d.Id())
	}

	post := mock.FindRequest("POST", "/gitops/sources/git")
	if post == nil {
		t.Fatal("expected POST /gitops/sources/git")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["url"] != "https://github.com/acme/infra.git" || payload["name"] != "infra" {
		t.Errorf("payload identity mismatch: %v", payload)
	}
	if payload["tlsSkipVerify"] != true || payload["administratorsOnly"] != true {
		t.Errorf("payload flags mismatch: %v", payload)
	}
	auth, ok := payload["authentication"].(map[string]interface{})
	if !ok || auth["username"] != "ci" || auth["password"] != "token" {
		t.Errorf("payload.authentication mismatch: %v", payload["authentication"])
	}
	if len(payload["userAccesses"].([]interface{})) != 2 {
		t.Errorf("payload.userAccesses: got %v", payload["userAccesses"])
	}

	// State comes from the follow-up read.
	if got := d.Get("status"); got != "healthy" {
		t.Errorf("status: got %v", got)
	}
	if got := d.Get("last_sync"); got != 1756900000 {
		t.Errorf("last_sync: got %v", got)
	}
	if got := d.Get("username"); got != "ci" {
		t.Errorf("username should come back from the API, got %v", got)
	}
}

// TestGitopsSourceCreate_NoIDReturned verifies a response without an ID fails
// loudly instead of leaving a resource pointing at source 0.
func TestGitopsSourceCreate_NoIDReturned(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/sources/git", RespondJSON(http.StatusCreated, map[string]interface{}{}))

	r := resourceGitopsSource()
	d := r.TestResourceData()
	_ = d.Set("url", "https://github.com/acme/infra.git")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Create to fail when Portainer returns no source ID")
	}
	if d.Id() != "" {
		t.Errorf("id should stay empty, got %q", d.Id())
	}
}

// TestGitopsSourceUpdate_SplitsConnectionAndAccess verifies the two-endpoint
// update: the git connection goes to the source, access control to its own
// endpoint, and each is only called when its own fields changed.
func TestGitopsSourceUpdate_SplitsConnectionAndAccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/gitops/sources/12", RespondJSON(http.StatusOK, map[string]interface{}{"id": 12}))
	mock.On("PUT", "/gitops/sources/12/access", RespondJSON(http.StatusOK, map[string]interface{}{"id": 12}))
	mock.On("GET", "/gitops/sources/12", RespondJSON(http.StatusOK, gitopsSourceDetailBody(12)))

	r := resourceGitopsSource()
	d := gitopsSourceDataWithChanges(t, "12", map[string]string{
		"url":    "https://github.com/acme/infra.git",
		"public": "true",
	})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/gitops/sources/12")
	if put == nil {
		t.Fatal("expected PUT /gitops/sources/12 when the URL changed")
	}
	var conn map[string]interface{}
	if err := put.DecodeJSON(&conn); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if _, present := conn["public"]; present {
		t.Error("access control must not be sent on the source update payload")
	}

	access := mock.FindRequest("PUT", "/gitops/sources/12/access")
	if access == nil {
		t.Fatal("expected PUT /gitops/sources/12/access when public changed")
	}
	var acc map[string]interface{}
	if err := access.DecodeJSON(&acc); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if acc["public"] != true {
		t.Errorf("access payload.public: got %v", acc["public"])
	}
	if _, present := acc["url"]; present {
		t.Error("the access payload must not carry connection fields")
	}
}

// TestGitopsSourceRead_404ClearsID verifies a source deleted outside Terraform
// drops out of state instead of failing every plan.
func TestGitopsSourceRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/sources/99",
		RespondString(http.StatusNotFound, "application/json", `{"message":"source not found"}`))

	r := resourceGitopsSource()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should not fail on 404: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared, got %q", d.Id())
	}
}

// TestGitopsSourceDelete_HappyPath verifies the DELETE and that a source
// already gone is treated as success.
func TestGitopsSourceDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/gitops/sources/12", RespondString(http.StatusNoContent, "", ""))

	r := resourceGitopsSource()
	d := r.TestResourceData()
	d.SetId("12")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/gitops/sources/12") == nil {
		t.Error("expected DELETE /gitops/sources/12")
	}
}

func TestGitopsSourceDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/gitops/sources/13",
		RespondString(http.StatusNotFound, "application/json", `{"message":"source not found"}`))

	r := resourceGitopsSource()
	d := r.TestResourceData()
	d.SetId("13")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat a missing source as success: %v", err)
	}
}
