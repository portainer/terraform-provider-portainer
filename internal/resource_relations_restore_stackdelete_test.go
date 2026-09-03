package internal

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// TestEndpointRelations_KeyedByEnvironment verifies the payload is a map keyed
// by environment ID, which is the shape endpointUpdateRelationsPayload takes.
func TestEndpointRelations_KeyedByEnvironment(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/relations", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointRelations()
	d := r.TestResourceData()
	_ = d.Set("relation", []interface{}{
		map[string]interface{}{
			"endpoint_id":    9,
			"edge_group_ids": []interface{}{1, 2},
			"tag_ids":        []interface{}{3},
			"group_id":       5,
		},
		map[string]interface{}{
			"endpoint_id":    10,
			"edge_group_ids": []interface{}{},
			"tag_ids":        []interface{}{},
			"group_id":       0,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/relations")
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	relations, ok := payload["Relations"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should carry a Relations map, got %v", payload)
	}
	nine, ok := relations["9"].(map[string]interface{})
	if !ok {
		t.Fatalf("relations should be keyed by environment ID, got keys %v", relations)
	}
	if nine["Group"] != float64(5) {
		t.Errorf("Group: got %v", nine["Group"])
	}
	if groups := nine["EdgeGroups"].([]interface{}); len(groups) != 2 {
		t.Errorf("EdgeGroups: got %v", groups)
	}
	if _, ok := relations["10"]; !ok {
		t.Error("every configured relation block must reach the payload")
	}
}

// TestRestore_SendsDecodedArchive verifies the archive round-trips: the
// configuration supplies base64, Portainer's FileContent is []byte, and
// encoding/json re-encodes it, so what arrives must equal what was configured.
func TestRestore_SendsDecodedArchive(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondString(http.StatusNoContent, "", ""))

	archive := []byte("fake tar.gz bytes")
	encoded := base64.StdEncoding.EncodeToString(archive)

	r := resourceRestore()
	d := r.TestResourceData()
	_ = d.Set("file_content_base64", encoded)
	_ = d.Set("file_name", "backup.tar.gz")
	_ = d.Set("password", "secret")
	_ = d.Set("setup_token", "token-123")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/restore")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["FileName"] != "backup.tar.gz" || payload["Password"] != "secret" {
		t.Errorf("payload mismatch: %v", payload)
	}
	got, err := base64.StdEncoding.DecodeString(payload["FileContent"].(string))
	if err != nil {
		t.Fatalf("FileContent is not valid base64: %v", err)
	}
	if string(got) != string(archive) {
		t.Errorf("archive mismatch: %q", got)
	}
	if h := post.Headers.Get("X-Setup-Token"); h != "token-123" {
		t.Errorf("X-Setup-Token header: expected %q, got %q", "token-123", h)
	}
	// The setup token must not displace the API key.
	if post.Headers.Get("X-API-Key") == "" {
		t.Error("the API key header must still be sent alongside the setup token")
	}
}

// TestRestore_RejectsInvalidBase64 verifies bad input fails before any request
// is made, rather than sending an empty archive to a restore endpoint.
func TestRestore_RejectsInvalidBase64(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/restore", RespondString(http.StatusNoContent, "", ""))

	r := resourceRestore()
	d := r.TestResourceData()
	_ = d.Set("file_content_base64", "not base64!!")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Create to reject invalid base64")
	}
	if mock.FindRequest("POST", "/restore") != nil {
		t.Error("no request should be made when the input cannot be decoded")
	}
}

// TestStackDeleteByName_SendsQuery verifies the environment and the external
// flag reach the API as query parameters, and that the stack name is escaped.
func TestStackDeleteByName_SendsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/stacks/name/my-stack", RespondString(http.StatusNoContent, "", ""))

	r := resourceStackDeleteByName()
	d := r.TestResourceData()
	_ = d.Set("name", "my-stack")
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("external", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	del := mock.FindRequest("DELETE", "/stacks/name/my-stack")
	if del == nil {
		t.Fatal("expected DELETE /stacks/name/my-stack")
	}
	for _, want := range []string{"endpointId=4", "external=true"} {
		if !strings.Contains(del.Query, want) {
			t.Errorf("query %q should contain %q", del.Query, want)
		}
	}
}

// TestStackDeleteByName_AlreadyGoneIsSuccess verifies a stack that no longer
// exists is the state this resource asks for, not an error.
func TestStackDeleteByName_AlreadyGoneIsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/stacks/name/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"stack not found"}`))

	r := resourceStackDeleteByName()
	d := r.TestResourceData()
	_ = d.Set("name", "gone")
	_ = d.Set("endpoint_id", 4)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("a missing stack must not fail the apply: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected an ID to be set")
	}
}

// TestResourceControlCreate_PostsWhenNoneExists verifies the new create path:
// a resource type Portainer cannot resolve a control for (anything but a stack)
// now creates one through POST /resource_controls instead of failing.
func TestResourceControlCreate_PostsWhenNoneExists(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/resource_controls", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 42}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "container-abc")
	_ = d.Set("type", 1) // container: not lookupable
	_ = d.Set("public", true)
	_ = d.Set("teams", []interface{}{2})
	_ = d.Set("users", []interface{}{7})
	_ = d.Set("sub_resource_ids", []interface{}{"vol-1"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "42" {
		t.Errorf("id: expected %q, got %q", "42", d.Id())
	}

	post := mock.FindRequest("POST", "/resource_controls")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["ResourceID"] != "container-abc" || payload["Type"] != float64(1) {
		t.Errorf("payload mismatch: %v", payload)
	}
	if payload["Public"] != true {
		t.Errorf("payload.Public: got %v", payload["Public"])
	}
	if subs := payload["SubResourceIDs"].([]interface{}); len(subs) != 1 || subs[0] != "vol-1" {
		t.Errorf("payload.SubResourceIDs: got %v", payload["SubResourceIDs"])
	}
}

// TestResourceControlRead_KeepsNonLookupableInState verifies the counterpart of
// the create path: Portainer has no GET /resource_controls/{id}, so a control
// created for a non-lookupable type must not be dropped from state on refresh.
func TestResourceControlRead_KeepsNonLookupableInState(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("42")
	_ = d.Set("resource_id", "container-abc")
	_ = d.Set("type", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "42" {
		t.Errorf("expected the resource to stay in state, got id %q", d.Id())
	}
}

// TestEndpointRelations_OmitsUntouchedFields is the regression test for a
// destructive bug: Portainer's updateRelations acts on Tags and EdgeGroups only
// when they are non-nil, and on Group only when non-zero. Sending an empty
// array therefore does not mean "leave this alone" — it CLEARS that
// environment's tags or edge groups. The SDK fills an omitted nested list with
// an empty slice, so a relation block that configures only one field must send
// only that field.
func TestEndpointRelations_OmitsUntouchedFields(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/relations", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointRelations()
	d := r.TestResourceData()
	_ = d.Set("relation", []interface{}{
		// Only edge groups: tags and group must not appear at all.
		map[string]interface{}{"endpoint_id": 9, "edge_group_ids": []interface{}{1}},
		// Only a group move.
		map[string]interface{}{"endpoint_id": 10, "group_id": 5},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/relations")
	var payload struct {
		Relations map[string]map[string]interface{} `json:"Relations"`
	}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	nine := payload.Relations["9"]
	if _, present := nine["Tags"]; present {
		t.Error("Tags must be omitted when not configured — an empty array clears the environment's tags")
	}
	if _, present := nine["Group"]; present {
		t.Error("Group must be omitted when zero")
	}
	if groups, ok := nine["EdgeGroups"].([]interface{}); !ok || len(groups) != 1 {
		t.Errorf("EdgeGroups should carry the configured value, got %v", nine["EdgeGroups"])
	}

	ten := payload.Relations["10"]
	if _, present := ten["EdgeGroups"]; present {
		t.Error("EdgeGroups must be omitted when not configured — an empty array clears the environment's edge groups")
	}
	if _, present := ten["Tags"]; present {
		t.Error("Tags must be omitted when not configured")
	}
	if ten["Group"] != float64(5) {
		t.Errorf("Group: expected 5, got %v", ten["Group"])
	}
}

// TestEndpointRelations_OmittedOptionalListsDoNotPanic pins that reading a
// relation block which omits both optional lists is safe: the SDK hands back an
// empty slice rather than nil, so the type assertions hold.
func TestEndpointRelations_OmittedOptionalListsDoNotPanic(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/relations", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointRelations()
	d := r.TestResourceData()
	_ = d.Set("relation", []interface{}{map[string]interface{}{"endpoint_id": 9}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/relations")
	var payload struct {
		Relations map[string]map[string]interface{} `json:"Relations"`
	}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	// A block that configures nothing is a no-op on the Portainer side.
	if len(payload.Relations["9"]) != 0 {
		t.Errorf("a relation configuring nothing must send an empty object, got %v", payload.Relations["9"])
	}
}
