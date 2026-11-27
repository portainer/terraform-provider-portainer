package internal

import (
	"context"
	"net/http"
	"testing"
)

// =========================================================================
// Wave three, remainder: licensing, recommendations, policies, observability,
// Docker snapshots, image status, edge update schedules and the git helpers.
// =========================================================================

// TestAPIGETRaw_NotFoundIsRecognised checks the error type carries through, so
// callers that tolerate a missing resource still can.
func TestAPIGETRaw_NotFoundIsRecognised(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/docker/5/snapshot", RespondString(http.StatusNotFound,
		"application/json", `{"message":"environment not found"}`))

	_, err := apiGETRaw(context.Background(), mock.Client(), mock.Client().Endpoint+"/docker/5/snapshot")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !isAPINotFound(err) {
		t.Errorf("a 404 must be recognisable as a missing resource, got: %v", err)
	}
}
