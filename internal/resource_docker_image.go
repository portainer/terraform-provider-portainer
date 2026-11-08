package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type dockerImageAuth struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress"`
}

func resourceDockerImage() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerImageCreate,
		ReadContext:   resourceDockerImageRead,
		DeleteContext: resourceDockerImageDelete,
		UpdateContext: nil,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id":              {Type: schema.TypeInt, Required: true, ForceNew: true, Description: "Identifier of the Portainer endpoint on which the Docker image should be pulled."},
			"image":                    {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Image reference to pull (for example `nginx:latest`)."},
			"registry_auth":            {Type: schema.TypeString, Optional: true, Sensitive: true, ForceNew: true, ConflictsWith: []string{"registry_auth_wo"}, Description: "Registry credentials used to pull the image, as `username:password`. Stored in state as a sensitive value; use `registry_auth_wo` to keep it out of state."},
			"registry_auth_wo":         writeOnlySecretSchema("registry_auth", "registry_auth_wo_version", "registry credentials (`username:password`)"),
			"registry_auth_wo_version": writeOnlyVersionSchema("registry_auth_wo", true),
		},
	}
}

func resourceDockerImageCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	image := d.Get("image").(string)
	endpointID := d.Get("endpoint_id").(int)
	auth, err := twinSecret(d, "registry_auth")
	if err != nil {
		return diag.FromErr(err)
	}

	timeout := d.Timeout(schema.TimeoutCreate)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := url.Values{}
	params.Add("fromImage", image)
	urlPath := fmt.Sprintf("%s/endpoints/%d/docker/images/create?%s", client.Endpoint, endpointID, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlPath, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to build request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "0")

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	if auth != "" {
		split := strings.SplitN(auth, ":", 2)
		if len(split) != 2 {
			return diag.FromErr(fmt.Errorf("invalid registry_auth format (expected username:password)"))
		}
		payload := dockerImageAuth{
			Username:      split[0],
			Password:      split[1],
			Email:         "",
			ServerAddress: strings.Split(image, "/")[0],
		}
		jsonData, _ := json.Marshal(payload)
		encoded := base64.StdEncoding.EncodeToString(jsonData)
		req.Header.Set("X-Registry-Auth", encoded)
	} else {
		req.Header.Set("X-Registry-Auth", base64.StdEncoding.EncodeToString([]byte(`{}`)))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return diag.FromErr(fmt.Errorf("failed to pull image, status code: %d, body: %s", resp.StatusCode, string(body)))
	}
	log.Printf("[DEBUG] Docker image pull result: %s", string(body))

	d.SetId(fmt.Sprintf("%d-%s", endpointID, image))
	return nil
}

// resourceDockerImageRead checks the image is still on the endpoint, so one
// removed outside Terraform is pulled again.
func resourceDockerImageRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	image := d.Get("image").(string)

	inspectURL := fmt.Sprintf("%s/endpoints/%d/docker/images/%s/json", client.Endpoint, endpointID, url.PathEscape(image))
	if err := doJSON(ctx, client, http.MethodGet, inspectURL, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to inspect image %s: %w", image, err))
	}
	return nil
}

func resourceDockerImageDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	image := d.Get("image").(string)

	timeout := d.Timeout(schema.TimeoutDelete)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	urlPath := fmt.Sprintf("%s/endpoints/%d/docker/images/%s", client.Endpoint, endpointID, url.PathEscape(image))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, urlPath, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to build request: %w", err))
	}

	req.Header.Set("Content-Type", "application/json")

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return diag.FromErr(fmt.Errorf("failed to delete image, status code: %d, body: %s", resp.StatusCode, string(body)))
	}
	log.Printf("[DEBUG] Docker image delete result: %s", string(body))

	d.SetId("")
	return nil
}
