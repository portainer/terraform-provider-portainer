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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// portainer_docker_image pulls an image onto an environment, and removes it
// on destroy. Every attribute forces a new pull.

var newDockerImageResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_image",
	schema:     dockerImageSchema,
	create:     dockerImageCreate,
	read:       dockerImageRead,
	delete:     dockerImageDelete,
	timeouts:   map[string]time.Duration{"create": 10 * time.Minute, "delete": 5 * time.Minute},
})

func dockerImageSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"endpoint_id":              schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint on which the Docker image should be pulled.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"image":                    schema.StringAttribute{Required: true, Description: "Image reference to pull (for example `nginx:latest`).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"registry_auth":            schema.StringAttribute{Optional: true, Sensitive: true, Description: "Registry credentials used to pull the image, as `username:password`. Stored in state as a sensitive value; use `registry_auth_wo` to keep it out of state.", PlanModifiers: []planmodifier.String{dkReplaceString()}, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("registry_auth_wo"))}},
			"registry_auth_wo":         schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Description: "Write-only registry credentials (`username:password`) (supports ephemeral values; never stored in Terraform state). Conflicts with `registry_auth`. Requires Terraform 1.11 or later.", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("registry_auth")), stringvalidator.AlsoRequires(path.MatchRoot("registry_auth_wo_version"))}},
			"registry_auth_wo_version": schema.Int64Attribute{Optional: true, Description: "Version of `registry_auth_wo`. Increment it to send a new value. Changing it forces resource recreation.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

type dockerImageAuth struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress"`
}

func dockerImageCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	image := dkString(d, "image")
	endpointID := dkInt(d, "endpoint_id")
	auth, err := dkTwinSecret(d, "registry_auth")
	if err != nil {
		return err
	}

	timeout := d.Timeout("create", 10*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := url.Values{}
	params.Add("fromImage", image)
	urlPath := fmt.Sprintf("%s/endpoints/%d/docker/images/create?%s", client.Endpoint, endpointID, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlPath, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
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
			return fmt.Errorf("invalid registry_auth format (expected username:password)")
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
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to pull image, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	log.Printf("[DEBUG] Docker image pull result: %s", string(body))

	d.SetID(fmt.Sprintf("%d-%s", endpointID, image))
	return nil
}

// dockerImageRead checks the image is still on the endpoint, so one
// removed outside Terraform is pulled again.
func dockerImageRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	image := dkString(d, "image")

	inspectURL := fmt.Sprintf("%s/endpoints/%d/docker/images/%s/json", client.Endpoint, endpointID, url.PathEscape(image))
	if err := doJSON(ctx, client, http.MethodGet, inspectURL, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to inspect image %s: %w", image, err)
	}
	return nil
}

func dockerImageDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	image := dkString(d, "image")

	timeout := d.Timeout("delete", 5*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	urlPath := fmt.Sprintf("%s/endpoints/%d/docker/images/%s", client.Endpoint, endpointID, url.PathEscape(image))

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, urlPath, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete image, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	log.Printf("[DEBUG] Docker image delete result: %s", string(body))

	d.SetID("")
	return nil
}
