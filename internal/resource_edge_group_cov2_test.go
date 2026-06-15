package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestEdgeGroupCov2_Update_HappyPath drives Update directly (PUT /edge_groups/{id})
// and asserts the payload carries endpoints + tagIDs, then chains into Read.
func TestEdgeGroupCov2_Update_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_groups/9", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))
	mock.On("GET", "/edge_groups/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":         "grp",
		"Dynamic":      false,
		"PartialMatch": true,
		"TagIds":       []int{1},
		"Endpoints":    []int{20},
	}))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("name", "grp")
	_ = d.Set("dynamic", false)
	_ = d.Set("partial_match", true)
	_ = d.Set("endpoints", []interface{}{20})
	_ = d.Set("tag_ids", []interface{}{1})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_groups/9")
	if put == nil {
		t.Fatal("expected PUT /edge_groups/9")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if got := payload["name"]; got != "grp" {
		t.Errorf("payload.name: got %v", got)
	}
	if got := payload["partialMatch"]; got != true {
		t.Errorf("payload.partialMatch: expected true, got %v", got)
	}
	// endpoints + tagIDs present because GetOk returned true (non-empty).
	if _, ok := payload["endpoints"]; !ok {
		t.Error("expected endpoints in payload")
	}
	if _, ok := payload["tagIDs"]; !ok {
		t.Error("expected tagIDs in payload")
	}
}

// TestEdgeGroupCov2_Update_HTTPError covers the non-200 branch of Update.
func TestEdgeGroupCov2_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_groups/9", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("name", "grp")
	_ = d.Set("dynamic", false)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeGroupCov2_Read_HTTPError covers the non-404 error branch of Read.
func TestEdgeGroupCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups/9", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeGroupCov2_Delete_HTTPError covers the non-204 branch of Delete.
func TestEdgeGroupCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_groups/9", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on non-204 delete, got nil")
	}
}

// TestEdgeGroupCov2_Create_ListError covers the findExistingEdgeGroupByName
// failure path (list GET returns non-200), surfaced as a create error.
func TestEdgeGroupCov2_Create_ListError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("dynamic", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when listing existing groups fails, got nil")
	}
}

// TestEdgeGroupCov2_FindExistingByName covers findExistingEdgeGroupByName directly.
func TestEdgeGroupCov2_FindExistingByName(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "prod"},
	}))
	client := mock.Client()

	id, err := findExistingEdgeGroupByName(context.Background(), client, "prod")
	if err != nil {
		t.Fatalf("findExistingEdgeGroupByName: %v", err)
	}
	if id != 7 {
		t.Errorf("expected id 7, got %d", id)
	}

	id, err = findExistingEdgeGroupByName(context.Background(), client, "nope")
	if err != nil {
		t.Fatalf("findExistingEdgeGroupByName: %v", err)
	}
	if id != 0 {
		t.Errorf("expected id 0 for missing name, got %d", id)
	}
}

// TestEdgeGroupCov2_BuildPayload covers buildEdgeGroupPayload with and without
// the optional endpoints/tag_ids set.
func TestEdgeGroupCov2_BuildPayload(t *testing.T) {
	t.Run("with optional lists", func(t *testing.T) {
		r := resourceEdgeGroup()
		d := r.TestResourceData()
		_ = d.Set("name", "g")
		_ = d.Set("dynamic", true)
		_ = d.Set("partial_match", true)
		_ = d.Set("endpoints", []interface{}{1, 2})
		_ = d.Set("tag_ids", []interface{}{3})

		p := buildEdgeGroupPayload(d)
		if p["name"] != "g" || p["dynamic"] != true || p["partialMatch"] != true {
			t.Errorf("unexpected base fields: %+v", p)
		}
		if _, ok := p["endpoints"]; !ok {
			t.Error("expected endpoints key present")
		}
		if _, ok := p["tagIDs"]; !ok {
			t.Error("expected tagIDs key present")
		}
	})

	t.Run("without optional lists", func(t *testing.T) {
		r := resourceEdgeGroup()
		d := r.TestResourceData()
		_ = d.Set("name", "g")
		_ = d.Set("dynamic", false)

		p := buildEdgeGroupPayload(d)
		if _, ok := p["endpoints"]; ok {
			t.Error("did not expect endpoints key when unset")
		}
		if _, ok := p["tagIDs"]; ok {
			t.Error("did not expect tagIDs key when unset")
		}
	})
}
