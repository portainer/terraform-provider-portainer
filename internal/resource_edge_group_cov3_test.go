package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestEdgeGroupCov3_Create_ResponseDecodeError covers the branch where the POST
// /edge_groups response is a 2xx but carries an undecodable body, so the
// json.Decode of the {Id} result fails and Create returns an error.
func TestEdgeGroupCov3_Create_ResponseDecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_groups", RespondString(http.StatusOK, "application/json", `not-json`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "g")
	_ = d.Set("dynamic", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed create response, got nil")
	}
}

// TestEdgeGroupCov3_Read_DecodeError covers the json.Decode failure branch of
// Read: a 200 response with a body that is not a valid group object.
func TestEdgeGroupCov3_Read_DecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups/9", RespondString(http.StatusOK, "application/json", `not-json`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed read response, got nil")
	}
}

// TestEdgeGroupCov3_FindExisting_DecodeError covers the decode failure branch of
// findExistingEdgeGroupByName when the list endpoint returns malformed JSON.
func TestEdgeGroupCov3_FindExisting_DecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_groups", RespondString(http.StatusOK, "application/json", `not-json`))

	if _, err := findExistingEdgeGroupByName(context.Background(), mock.Client(), "x"); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
