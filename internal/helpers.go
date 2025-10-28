package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

func parseManifest(manifest string) (map[string]interface{}, error) {
	var parsed map[string]interface{}

	// Try JSON
	if err := json.Unmarshal([]byte(manifest), &parsed); err == nil {
		return parsed, nil
	}

	// Try YAML
	if err := yaml.Unmarshal([]byte(manifest), &parsed); err == nil {
		return parsed, nil
	}

	return nil, fmt.Errorf("manifest is neither valid JSON nor YAML")
}

func toIntSlice(raw []interface{}) []int {
	res := make([]int, len(raw))
	for i, v := range raw {
		res[i] = v.(int)
	}
	return res
}

func apiGET(url string, apiKey string, client *APIClient) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func splitAndTrimCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func contains(arr []string, v string) bool {
	for _, x := range arr {
		if x == v {
			return true
		}
	}
	return false
}

func mustMap(v interface{}) map[string]interface{} {
	if v == nil {
		m := make(map[string]interface{})
		return m
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func apiGETWithCode(url string, apiKey string, client *APIClient) ([]byte, int, error) {
	req, _ := http.NewRequest("GET", url, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func apiPOSTWithCode(url string, apiKey string, client *APIClient, payload []byte) ([]byte, int, error) {
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func apiPUTWithCode(url string, apiKey string, client *APIClient, payload []byte) ([]byte, int, error) {
	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(payload))
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}
