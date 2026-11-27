package internal

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// accAPIGet sends GET <PORTAINER_ENDPOINT>/api<path> with the acceptance API
// key, for acceptance tests that skip when the server lacks a feature. It
// returns the status and the body; a transport error fails the test.
func accAPIGet(t *testing.T, path string) (int, []byte) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests need TF_ACC=1")
	}
	accPreCheck(t)
	base := strings.TrimSuffix(os.Getenv("PORTAINER_ENDPOINT"), "/")
	if !strings.HasSuffix(base, "/api") {
		base += "/api"
	}
	req, err := http.NewRequest(http.MethodGet, base+path, nil) //nolint:gosec // the URL is the acceptance instance from PORTAINER_ENDPOINT
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	req.Header.Set("X-API-Key", os.Getenv("PORTAINER_API_KEY"))
	client := &http.Client{
		Timeout: 30 * time.Second,
		// The acceptance instances use a self-signed certificate.
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req) //nolint:gosec // the URL is the acceptance instance from PORTAINER_ENDPOINT
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// accSkipUnlessAPI skips an acceptance test when GET path answers 404: the
// server does not have the endpoint.
func accSkipUnlessAPI(t *testing.T, path, why string) {
	t.Helper()
	if status, _ := accAPIGet(t, path); status == http.StatusNotFound {
		t.Skipf("GET %s returned 404: %s", path, why)
	}
}

// accSystemInfo reads GET /system/info (field addonsAvailable, among others).
func accSystemInfo(t *testing.T) map[string]interface{} {
	t.Helper()
	status, body := accAPIGet(t, "/system/info")
	info := map[string]interface{}{}
	if status == http.StatusOK {
		_ = json.Unmarshal(body, &info)
	}
	return info
}
