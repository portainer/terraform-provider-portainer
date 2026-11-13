package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2.45.3", "3.0.0", true},
		{"3.0.0", "3.0.0", false},
		{"3.0.1", "3.0.0", false},
		{"3.0", "3.0.0", false},
		{"3.0.0-rc1", "3.0.0", false},
		{"2.9.0", "2.10.0", true},
	}
	for _, tc := range cases {
		if got := versionLess(tc.a, tc.b); got != tc.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestExplainUnsupported: a 404 from a server too old or of the wrong edition
// names the requirement; on a server that meets it, the error is unchanged.
func TestExplainUnsupported(t *testing.T) {
	notFound := &apiStatusError{Method: "POST", URL: "http://x/api/endpoints/d2k", StatusCode: http.StatusNotFound, Body: "404 page not found"}
	cases := map[string]struct {
		version, edition string
		explained        bool
	}{
		"community edition":  {"2.45.3", "CE", true},
		"old business":       {"2.45.3", "BE", true},
		"business 3.0":       {"3.0.0", "BE", false},
		"business 3.0 as EE": {"3.0.1", "EE", false},
		"old EE":             {"2.45.3", "EE", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
				"ServerVersion": tc.version, "ServerEdition": tc.edition,
			}))
			err := explainUnsupported(context.Background(), mock.Client(), notFound, "portainer_d2k_environment", requiresBE30)
			explained := strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later")
			if explained != tc.explained {
				t.Errorf("explained = %v, want %v: %v", explained, tc.explained, err)
			}
			if explained && !strings.Contains(err.Error(), tc.edition+" "+tc.version) {
				t.Errorf("the error should name the server, got %v", err)
			}
		})
	}

	t.Run("other errors untouched", func(t *testing.T) {
		mock := NewMockServer(t)
		forbidden := fmt.Errorf("wrapped: %w", &apiStatusError{StatusCode: http.StatusForbidden})
		if err := explainUnsupported(context.Background(), mock.Client(), forbidden, "x", requiresBE30); !errors.Is(err, forbidden) {
			t.Errorf("a 403 must be returned unchanged, got %v", err)
		}
		if len(mock.Requests()) != 0 {
			t.Error("the version must only be asked for after a 404")
		}
	})
}
