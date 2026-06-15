package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var edgeConfigTypeToString = map[int]string{
	1: "general",
}

func edgeConfigTypeDiffSuppress(_, old, new string, _ *schema.ResourceData) bool {
	if oldInt, err := strconv.Atoi(old); err == nil {
		if name, ok := edgeConfigTypeToString[oldInt]; ok {
			return name == new
		}
	}
	if newInt, err := strconv.Atoi(new); err == nil {
		if name, ok := edgeConfigTypeToString[newInt]; ok {
			return name == old
		}
	}
	return old == new
}

type EdgeConfiguration struct {
	ID           int         `json:"id"`
	Name         string      `json:"name"`
	Type         int         `json:"type"`
	Category     string      `json:"category"`
	BaseDir      string      `json:"baseDir"`
	EdgeGroupIDs []int       `json:"edgeGroupIDs"`
	Created      int64       `json:"created"`
	CreatedBy    int         `json:"createdBy"`
	Updated      int64       `json:"updated"`
	UpdatedBy    int         `json:"updatedBy"`
	Prev         interface{} `json:"prev"`
}

func resourcePortainerEdgeConfigurations() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerEdgeConfigurationsCreate,
		ReadContext:   resourcePortainerEdgeConfigurationsRead,
		UpdateContext: resourcePortainerEdgeConfigurationsUpdate,
		DeleteContext: resourcePortainerEdgeConfigurationsDelete,
		CustomizeDiff: customizeDiffEdgeConfigurationFileHash,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true, ForceNew: true, ValidateFunc: validation.NoZeroValues, Description: "Name of the Portainer edge configuration."},
			"type":           {Type: schema.TypeString, Required: true, DiffSuppressFunc: edgeConfigTypeDiffSuppress, Description: "Edge configuration type. Accepts the textual form `general` or the numeric form `1`."},
			"category":       {Type: schema.TypeString, Optional: true, Default: "", ForceNew: true, ValidateFunc: validation.StringInSlice([]string{"configuration", "secret", ""}, false), Description: "Category of the edge configuration. One of `configuration`, `secret`, or empty string."},
			"base_dir":       {Type: schema.TypeString, Optional: true, Default: "", Description: "Base directory on edge agents where the configuration files are written."},
			"edge_group_ids": {Type: schema.TypeList, Required: true, Elem: &schema.Schema{Type: schema.TypeInt}, Description: "List of edge group identifiers that should receive this configuration."},
			"file_path":      {Type: schema.TypeString, Required: true, Description: "Path on the local filesystem to the file or archive uploaded as the edge configuration payload."},
			"file_sha256":    {Type: schema.TypeString, Computed: true, Description: "SHA256 hash of the uploaded edge configuration file used to detect content changes."},
		},
	}
}

func convertToIntSlice(input []interface{}) []int {
	result := make([]int, len(input))
	for i, v := range input {
		result[i] = v.(int)
	}
	return result
}

// sha256File returns the lowercase hex-encoded SHA256 of the file contents
// at the given path. Used to detect changes to the underlying edge config
// file when the file_path itself hasn't changed (issue #116) — the Portainer
// API doesn't expose the uploaded file content or any digest, so we track
// our locally computed hash in state.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// customizeDiffEdgeConfigurationFileHash hashes the file at file_path during
// plan and writes it to file_sha256. If the new hash differs from the value
// in state, Terraform sees a diff and triggers an in-place Update — even when
// file_path didn't change (i.e. the file's contents were rewritten in place).
func customizeDiffEdgeConfigurationFileHash(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	fp, _ := d.Get("file_path").(string)
	if fp == "" {
		return nil
	}
	hash, err := sha256File(fp)
	if err != nil {
		return fmt.Errorf("failed to hash file_path %q: %w", fp, err)
	}
	if d.Get("file_sha256").(string) == hash {
		return nil
	}
	return d.SetNew("file_sha256", hash)
}

// listEdgeConfigurations fetches all edge configurations from Portainer.
func listEdgeConfigurations(ctx context.Context, client *APIClient) ([]EdgeConfiguration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_configurations", client.Endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build list request: %w", err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge configurations: %w", err)
	}
	defer resp.Body.Close()
	var configs []EdgeConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return nil, fmt.Errorf("failed to decode edge configurations list: %w", err)
	}
	return configs, nil
}

// resolveCreatedEdgeConfigID disambiguates the just-created edge configuration
// when the Portainer API returns an empty POST response. It diffs the post-create
// listing against a pre-create snapshot of IDs sharing the same name; if no new
// entry is found, it falls back to the most recently created matching config.
//
// Background: Portainer's POST /edge_configurations does not return the new ID
// (https://github.com/portainer/terraform-provider-portainer/issues/115). Using
// name alone causes the provider to bind to a pre-existing same-name config,
// later mutating or deleting it. The snapshot-based diff fixes that for any
// case where a same-name config already exists.
func resolveCreatedEdgeConfigID(configs []EdgeConfiguration, name string, preExistingIDs map[int]struct{}) (EdgeConfiguration, error) {
	var newMatches []EdgeConfiguration
	allMatches := make([]EdgeConfiguration, 0, len(configs))
	for _, c := range configs {
		if c.Name != name {
			continue
		}
		allMatches = append(allMatches, c)
		if _, existed := preExistingIDs[c.ID]; !existed {
			newMatches = append(newMatches, c)
		}
	}

	pickNewest := func(in []EdgeConfiguration) EdgeConfiguration {
		sort.Slice(in, func(i, j int) bool { return in[i].Created > in[j].Created })
		return in[0]
	}

	switch {
	case len(newMatches) >= 1:
		// Exactly the entry POST just produced; if a concurrent caller raced
		// us, prefer the most recently created one.
		return pickNewest(newMatches), nil
	case len(allMatches) >= 1:
		// Server returned no new entry (replication lag, server quirk). Best
		// effort: pick the most recently created matching name.
		return pickNewest(allMatches), nil
	default:
		return EdgeConfiguration{}, fmt.Errorf("edge configuration created but could not determine its ID")
	}
}

func resourcePortainerEdgeConfigurationsCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	// Snapshot existing edge configuration IDs sharing the requested name.
	// Portainer's POST does not return the new ID, so after the create we
	// diff the listing against this snapshot to identify the new entry. This
	// is what prevents adopting a pre-existing same-name config (issue #115).
	preCreate, err := listEdgeConfigurations(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}
	preExistingIDs := make(map[int]struct{})
	for _, c := range preCreate {
		if c.Name == name {
			preExistingIDs[c.ID] = struct{}{}
		}
	}

	filePath := d.Get("file_path").(string)
	file, err := os.Open(filePath)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to open file: %w", err))
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	payload := map[string]interface{}{
		"name":         name,
		"type":         d.Get("type").(string),
		"category":     d.Get("category").(string),
		"baseDir":      d.Get("base_dir").(string),
		"edgeGroupIDs": convertToIntSlice(d.Get("edge_group_ids").([]interface{})),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to marshal edgeConfiguration payload: %w", err))
	}
	_ = writer.WriteField("edgeConfiguration", string(payloadBytes))

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create form file: %w", err))
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to copy file content: %w", err))
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/edge_configurations", client.Endpoint), body)
	if err != nil {
		return diag.FromErr(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create edge configuration: %s", string(respBody)))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read create response: %w", err))
	}

	var created EdgeConfiguration
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &created); err != nil {
			return diag.FromErr(fmt.Errorf("failed to decode create response: %w", err))
		}
	}

	if created.ID == 0 {
		postCreate, err := listEdgeConfigurations(ctx, client)
		if err != nil {
			return diag.FromErr(err)
		}
		created, err = resolveCreatedEdgeConfigID(postCreate, name, preExistingIDs)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(strconv.Itoa(created.ID))

	// Persist the file's SHA256 in state so future plans can detect content
	// changes even when file_path is unchanged (issue #116). CustomizeDiff
	// already computed this for the diff, but we re-compute here so the value
	// stored matches the bytes we actually uploaded.
	if hash, err := sha256File(filePath); err == nil {
		if err := d.Set("file_sha256", hash); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourcePortainerEdgeConfigurationsUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	rawID := filepath.Base(d.Id())

	filePath := d.Get("file_path").(string)
	file, err := os.Open(filePath)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to open file: %w", err))
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Portainer API docs are incorrect. API expects form data as follows,
	// note lower case naming as well:
	// edgeConfiguration: {"edgeGroupIDs":[...],"type":"..."}
	// file:              (binary file)
	payload := map[string]interface{}{
		"type":         d.Get("type").(string),
		"edgeGroupIDs": convertToIntSlice(d.Get("edge_group_ids").([]interface{})),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to marshal edgeConfiguration payload: %w", err))
	}
	_ = writer.WriteField("edgeConfiguration", string(payloadBytes))

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create form file: %w", err))
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to copy file content: %w", err))
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID), body)
	if err != nil {
		return diag.FromErr(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update edge configuration: %s", string(respBody)))
	}

	if hash, err := sha256File(filePath); err == nil {
		if err := d.Set("file_sha256", hash); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourcePortainerEdgeConfigurationsRead(ctx, d, meta)
}

func resourcePortainerEdgeConfigurationsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	id := d.Id()
	rawID := filepath.Base(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID), nil)
	if err != nil {
		return diag.FromErr(err)
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}
	res, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return diag.FromErr(fmt.Errorf("failed to read edge configuration: %s", string(body)))
	}

	var config EdgeConfiguration
	if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", config.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("category", config.Category); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("base_dir", config.BaseDir); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("edge_group_ids", config.EdgeGroupIDs); err != nil {
		return diag.FromErr(err)
	}
	if typeName, ok := edgeConfigTypeToString[config.Type]; ok {
		if err := d.Set("type", typeName); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := d.Set("type", strconv.Itoa(config.Type)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourcePortainerEdgeConfigurationsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	rawID := filepath.Base(d.Id())
	url := fmt.Sprintf("%s/edge_configurations/%s", client.Endpoint, rawID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete edge configuration: %s", string(body)))
	}

	d.SetId("")
	return nil
}
