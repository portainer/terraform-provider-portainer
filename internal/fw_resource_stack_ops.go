package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// CRUD functions of portainer_stack (see fw_resource_stack.go). They keep the
// API calls, payloads, waits and error texts of the SDKv2 implementation.

// stackUseConfiguredContent makes stack_file_content what the configuration
// says, "" when it leaves it out. The attribute is computed so that Read can
// keep the deployed content in state, but the deploy calls send what SDKv2
// sent: the configured content, or the file read at apply (method "file").
func stackUseConfiguredContent(d *dkBag) {
	d.vals["stack_file_content"] = d.ConfigString("stack_file_content")
}

// stackApplyUpdate is the update entry point of the resource.
func stackApplyUpdate(ctx context.Context, d *dkBag, client *APIClient) error {
	stackUseConfiguredContent(d)
	return stackUpdate(ctx, d, client)
}

func findExistingStackByName(ctx context.Context, client *APIClient, name string, endpointID int) (int, error) {
	url := fmt.Sprintf("%s/stacks", client.Endpoint)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err := setAuthHeader(req, client); err != nil {
		return 0, err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list stacks: %s", string(data))
	}

	var stacks []struct {
		ID         int    `json:"Id"`
		Name       string `json:"Name"`
		EndpointID int    `json:"EndpointId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return 0, err
	}

	for _, s := range stacks {
		if s.Name == name && s.EndpointID == endpointID {
			return s.ID, nil
		}
	}
	return 0, nil // not found
}

func stackCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	stackUseConfiguredContent(d)
	deployment := dkString(d, "deployment_type")
	method := dkString(d, "method")
	name := dkString(d, "name")
	endpointID := dkInt(d, "endpoint_id")

	if deployment == "swarm" && d.Get("swarm_id") == "" {
		swarmID, err := fetchSwarmID(ctx, client, endpointID)
		if err != nil {
			return fmt.Errorf("failed to fetch swarm_id: %w", err)
		}
		if err := d.Set("swarm_id", swarmID); err != nil {
			return err
		}
	}

	if existingID, err := findExistingStackByName(ctx, client, name, endpointID); err != nil {
		return fmt.Errorf("error checking for existing stack: %w", err)
	} else if existingID != 0 {
		d.SetID(strconv.Itoa(existingID))
		return stackUpdate(ctx, d, client)
	}

	var err error

	switch deployment {
	case "standalone":
		switch method {
		case "string":
			err = createStackStandaloneString(ctx, d, client)
		case "file":
			path := dkString(d, "stack_file_path")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read stack file from path: %w", readErr)
			}
			if err := d.Set("stack_file_content", string(content)); err != nil {
				return err
			}
			err = createStackStandaloneString(ctx, d, client)
		case "repository":
			err = createStackStandaloneRepo(ctx, d, client)
		default:
			return fmt.Errorf("invalid method %q for standalone deployment", method)
		}

	case "swarm":
		switch method {
		case "string":
			err = createStackSwarmString(ctx, d, client)
		case "file":
			path := dkString(d, "stack_file_path")
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read stack file from path: %w", readErr)
			}
			if err := d.Set("stack_file_content", string(content)); err != nil {
				return err
			}
			err = createStackSwarmString(ctx, d, client)
		case "repository":
			err = createStackSwarmRepo(ctx, d, client)
		default:
			return fmt.Errorf("invalid method %q for swarm deployment", method)
		}

	case "kubernetes":
		switch method {
		case "string":
			err = createStackK8sString(ctx, d, client)
		case "repository":
			err = createStackK8sRepo(ctx, d, client)
		case "url":
			err = createStackK8sURL(ctx, d, client)
		default:
			return fmt.Errorf("invalid method %q for kubernetes deployment", method)
		}

	default:
		return fmt.Errorf("invalid deployment_type %q", deployment)
	}

	if err != nil {
		return err
	}

	if id := d.ID(); id == "" || id == "0" {
		resolvedID, lookupErr := findExistingStackByName(ctx, client, name, endpointID)
		if lookupErr != nil {
			return fmt.Errorf("stack %q was created but its ID could not be resolved by name on endpoint %d: %w", name, endpointID, lookupErr)
		}
		if resolvedID == 0 {
			return fmt.Errorf("stack %q was created but could not be found by name on endpoint %d to resolve its ID", name, endpointID)
		}
		d.SetID(strconv.Itoa(resolvedID))
	}

	if method != "repository" {
		var webhookToken string
		if dkBool(d, "stack_webhook") {
			webhookToken = dkString(d, "webhook_id")
			if webhookToken == "" {
				webhookToken = uuid.New().String()
			}
		}

		payload := map[string]interface{}{
			"env":              stackEnvPayload(d),
			"stackFileContent": dkString(d, "stack_file_content"),
			"prune":            dkBool(d, "prune"),
			"pullImage":        dkBool(d, "pull_image"),
			"registries":       dkIntSlice(dkList(d, "registries")),
		}
		if webhookToken != "" {
			payload["webhook"] = webhookToken
		}
		addStackRepull(d, payload)

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal stack update (create) payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.ID(), endpointID)

		// Portainer deploys the stack asynchronously from 2.45 and holds it in
		// status Deploying meanwhile, rejecting this call with 409 until the
		// deploy finishes (issue #145). Wait for it to settle, and retry a
		// bounded number of times in case another client starts a deploy in the
		// window between the poll and this request.
		for attempt := 0; ; attempt++ {
			if err := waitForStackDeployment(ctx, client, d.ID()); err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
			if err != nil {
				return fmt.Errorf("failed to build stack update (create) request: %w", err)
			}

			if err := setAuthHeader(req, client); err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to perform stack update (create) request: %w", err)
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				break
			}
			if isStackDeploymentConflict(resp.StatusCode, string(data)) && attempt < maxStackConflictRetries {
				continue
			}
			return fmt.Errorf("failed to finalize stack creation (prune/webhook), status %d: %s", resp.StatusCode, string(data))
		}

		if webhookToken != "" {
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
			if err := d.SetFields(map[string]interface{}{
				"webhook_id":  webhookToken,
				"webhook_url": webhookURL,
			}); err != nil {
				return err
			}
		}
	}

	// Wait out the deployment the create (or, for the other methods, the
	// update above) started. A git stack used to return while Portainer was
	// still deploying it: a deployment that failed went unreported, and the
	// Read below saw the stack as not active yet, so the next plan proposed
	// active = true.
	if err := waitForStackDeployment(ctx, client, d.ID()); err != nil {
		return err
	}

	// ACCESS CONTROL UPDATE
	if err := updateStackAccessControl(ctx, d, client, d.ID()); err != nil {
		return fmt.Errorf("failed to update stack access control: %w", err)
	}

	// The Portainer API always deploys a new stack in the running state, so if
	// the user requested active = false we have to stop it right after creation.
	if !dkBool(d, "active") {
		if err := setStackActive(ctx, client, d.ID(), endpointID, false); err != nil {
			return err
		}
	}

	return stackRead(ctx, d, client)
}

func stackRead(ctx context.Context, d *dkBag, client *APIClient) error {
	stackID := d.ID()

	url := fmt.Sprintf("%s/stacks/%s", client.Endpoint, stackID)

	var stack struct {
		Name                string `json:"Name"`
		Status              int    `json:"Status"`
		Type                int    `json:"Type"`
		SourceID            int    `json:"SourceID"`
		SwarmID             string `json:"SwarmId"`
		Namespace           string `json:"namespace"`
		ComposeFmt          bool   `json:"composeFormat"`
		Webhook             string `json:"webhook"`
		EndpointID          int    `json:"EndpointId"`
		SupportRelativePath bool   `json:"supportRelativePath"`
		AutoUpdate          *struct {
			Interval       string `json:"Interval"`
			Webhook        string `json:"Webhook"`
			ForcePullImage bool   `json:"ForcePullImage"`
		} `json:"AutoUpdate,omitempty"`
		Env []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"Env"`
		Registries []int `json:"Registries"`

		Option struct {
			Prune bool `json:"prune"`
			// Portainer 3.0 and later. Older versions leave them out, and a
			// nil KubernetesPrune means pruning is on, so neither is read
			// back unless it is returned.
			KubernetesPrune      *bool `json:"KubernetesPrune"`
			KubernetesForceApply *bool `json:"KubernetesForceApply"`
		} `json:"Option"`

		GitConfig *struct {
			URL             string   `json:"URL"`
			ReferenceName   string   `json:"ReferenceName"`
			ConfigFilePath  string   `json:"ConfigFilePath"`
			AdditionalFiles []string `json:"AdditionalFiles"`
			TLSSkipVerify   bool     `json:"tlsskipVerify"`
			Authentication  struct {
				GitCredentialID int `json:"GitCredentialID"`
			} `json:"Authentication"`
		} `json:"gitConfig,omitempty"`

		HelmConfig *struct {
			ChartPath   string   `json:"chartPath"`
			ValuesFiles []string `json:"valuesFiles"`
			Atomic      bool     `json:"atomic"`
		} `json:"helmConfig,omitempty"`

		Portainer struct {
			ResourceControl struct {
				ID int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &stack); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read stack: %w", err)
	}

	if err := d.Set("name", stack.Name); err != nil {
		return err
	}
	// Portainer API: Status 1 = active, 2 = inactive
	if err := d.Set("active", stack.Status == 1); err != nil {
		return err
	}
	if err := d.Set("swarm_id", stack.SwarmID); err != nil {
		return err
	}
	if err := d.Set("namespace", stack.Namespace); err != nil {
		return err
	}
	if err := d.Set("compose_format", stack.ComposeFmt); err != nil {
		return err
	}

	var webhookToken string
	if stack.AutoUpdate != nil && stack.AutoUpdate.Webhook != "" {
		webhookToken = stack.AutoUpdate.Webhook
	} else if stack.Webhook != "" {
		webhookToken = stack.Webhook
	}

	if webhookToken != "" {
		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		if err := d.SetFields(map[string]interface{}{
			"stack_webhook": true,
			"webhook_id":    webhookToken,
			"webhook_url":   webhookURL,
		}); err != nil {
			return err
		}
	} else {
		if err := d.SetFields(map[string]interface{}{
			"stack_webhook": false,
			"webhook_id":    "",
			"webhook_url":   "",
		}); err != nil {
			return err
		}
	}

	method := dkString(d, "method")
	if method != "repository" {
		fileURL := fmt.Sprintf("%s/stacks/%s/file", client.Endpoint, stackID)
		fileReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
		if err := setAuthHeader(fileReq, client); err != nil {
			return err
		}

		fileResp, err := client.HTTPClient.Do(fileReq)
		if err != nil {
			return fmt.Errorf("failed to fetch stack file: %w", err)
		}
		defer fileResp.Body.Close()

		if fileResp.StatusCode >= 400 {
			body, _ := io.ReadAll(fileResp.Body)
			return fmt.Errorf("failed to fetch stack file, status: %d, body: %s", fileResp.StatusCode, string(body))
		}

		var fileContent struct {
			StackFileContent string `json:"StackFileContent"`
		}
		if err := json.NewDecoder(fileResp.Body).Decode(&fileContent); err != nil {
			return fmt.Errorf("failed to decode stack file content: %w", err)
		}

		if err := d.Set("stack_file_content", fileContent.StackFileContent); err != nil {
			return err
		}
	}

	// Env → Terraform. Portainer returns the env_wo variables with the rest;
	// they stay out of env so their values never reach the state.
	writeOnlyNames := map[string]bool{}
	for _, name := range envNames(dkList(d, "env_wo")) {
		writeOnlyNames[name] = true
	}
	tfEnvs := make([]map[string]interface{}, 0, len(stack.Env))
	for _, env := range stack.Env {
		if writeOnlyNames[env.Name] {
			continue
		}
		tfEnvs = append(tfEnvs, map[string]interface{}{
			"name":  env.Name,
			"value": env.Value,
		})
	}
	if err := d.Set("env", tfEnvs); err != nil {
		return err
	}
	fields := map[string]interface{}{
		"method":                method,
		"endpoint_id":           stack.EndpointID,
		"support_relative_path": stack.SupportRelativePath,
	}
	if stack.SourceID != 0 {
		fields["source_id"] = stack.SourceID
	}
	if method == "repository" && stack.GitConfig != nil {
		fields["tlsskip_verify"] = stack.GitConfig.TLSSkipVerify
		fields["repository_git_credential_id"] = stack.GitConfig.Authentication.GitCredentialID
		if stack.GitConfig.URL != "" {
			fields["repository_url"] = stack.GitConfig.URL
		}
		if stack.GitConfig.ReferenceName != "" {
			fields["repository_reference_name"] = stack.GitConfig.ReferenceName
		}
		if stack.GitConfig.ConfigFilePath != "" {
			fields["file_path_in_repository"] = stack.GitConfig.ConfigFilePath
		}
		if len(stack.GitConfig.AdditionalFiles) > 0 {
			fields["additional_files"] = stack.GitConfig.AdditionalFiles
		}
	}
	if stack.HelmConfig != nil && stack.HelmConfig.ChartPath != "" {
		fields["helm_chart_path"] = stack.HelmConfig.ChartPath
		if len(stack.HelmConfig.ValuesFiles) > 0 {
			fields["additional_helm_values_files"] = stack.HelmConfig.ValuesFiles
		}
		// atomic is read back only while it is configured: unset, it leaves
		// the choice to whatever Portainer has recorded.
		if stackAtomicTracked(d) {
			fields["atomic"] = stack.HelmConfig.Atomic
		}
	}
	if stack.AutoUpdate != nil {
		fields["pull_image"] = stack.AutoUpdate.ForcePullImage
		fields["update_interval"] = stack.AutoUpdate.Interval
	}
	if stack.Portainer.ResourceControl.ID != 0 {
		fields["resource_control_id"] = stack.Portainer.ResourceControl.ID
	}
	if dkString(d, "deployment_type") == "kubernetes" {
		if stack.Option.KubernetesPrune != nil {
			fields["kubernetes_prune"] = *stack.Option.KubernetesPrune
		}
		if stack.Option.KubernetesForceApply != nil {
			fields["kubernetes_force_apply"] = *stack.Option.KubernetesForceApply
		}
	}
	if err := d.SetFields(fields); err != nil {
		return err
	}

	if stack.Portainer.ResourceControl.ID != 0 {
		// Read Access Control
		rcID := strconv.Itoa(stack.Portainer.ResourceControl.ID)
		if err := readStackAccessControl(ctx, d, client, rcID); err != nil {
			return fmt.Errorf("failed to read stack access control: %w", err)
		}
	}

	return nil
}

func fetchSwarmID(ctx context.Context, client *APIClient, endpointID int) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err := setAuthHeader(req, client); err != nil {
		return "", err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch swarm info: %s", string(data))
	}

	var swarm struct {
		ID string `json:"ID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&swarm); err != nil {
		return "", err
	}
	return swarm.ID, nil
}

func stackDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	id := d.ID()
	endpointID := dkInt(d, "endpoint_id")

	timeout := d.Timeout("delete", 10*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	retryInterval := 15 * time.Second

	for {
		deleteURL := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, id, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		if err != nil {
			return err
		}
		if err := setAuthHeader(req, client); err != nil {
			return err
		}

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		// Retry on server errors (5xx), except the ones that cannot go away
		// by waiting, which used to keep a destroy busy until the timeout.
		if resp.StatusCode >= 500 && !isPermanentStackDeleteError(string(body)) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout deleting stack %s after %s: last error: %s", id, timeout, string(body))
			case <-time.After(retryInterval):
				continue
			}
		}

		return fmt.Errorf("failed to delete stack: %s", string(body))
	}
}

// permanentStackDeleteErrors are Docker errors a stack deletion keeps getting
// whatever the wait: the swarm the stack belongs to is gone or this node is
// not one of its managers.
var permanentStackDeleteErrors = []string{
	"not a swarm manager",
	"not part of a swarm",
}

func isPermanentStackDeleteError(body string) bool {
	for _, msg := range permanentStackDeleteErrors {
		if strings.Contains(body, msg) {
			return true
		}
	}
	return false
}

// Portainer stack status values. Deploying and Error only exist from 2.45,
// where stack deployment became asynchronous; older versions never report
// anything but Active or Inactive.
const (
	stackStatusDeploying = 3
	stackStatusError     = 4
)

// stackSettlePollInterval is how often waitForStackSettled asks Portainer
// whether the deployment has finished. It is a variable so tests do not have to
// wait in real time.
var stackSettlePollInterval = 2 * time.Second

// maxStackConflictRetries bounds how many times a mutating stack call is
// retried after Portainer reports a deployment conflict. One retry covers the
// window between the readiness poll and the call itself; a handful also covers
// a second client starting a deploy inside that window.
const maxStackConflictRetries = 3

// waitForStackDeployment waits out a deployment this provider just started and
// reports a failed one as an error.
//
// Use it after a call of ours put the stack into Deploying: there, status 4
// (Error) is the outcome of that very deployment, and reporting success for a
// stack that did not come up would be a lie.
func waitForStackDeployment(ctx context.Context, client *APIClient, stackID string) error {
	_, err := waitForStackSettled(ctx, client, stackID, true)
	return err
}

// waitForStackLockReleased waits only while a deployment is in flight, so that
// a mutation of ours can be sent.
//
// Use it before a call of ours. A stack sitting in status 4 from an earlier
// deployment - a previous apply, a webhook, a redeploy in the UI - is not a
// reason to refuse: the call about to be sent is exactly what replaces that
// failed definition. Treating it as fatal left a failed stack unfixable through
// Terraform, because Portainer keeps the status until something deploys the
// stack again (issue #147).
func waitForStackLockReleased(ctx context.Context, client *APIClient, stackID string) error {
	_, err := waitForStackSettled(ctx, client, stackID, false)
	return err
}

// waitForStackIdle is waitForStackLockReleased for a call that redeploys the
// stack: it also returns when the stack's last deployment started, for
// waitForStackRedeployed to tell that deployment from the one the call starts.
func waitForStackIdle(ctx context.Context, client *APIClient, stackID string) (int64, error) {
	stack, err := waitForStackSettled(ctx, client, stackID, false)
	return stack.deploymentStart(), err
}

// waitForStackRedeployed waits out the redeploy started by a call of ours and
// reports it if it failed. A stack in Error is only blamed on that redeploy
// when its deployment history did not start before before, as returned by
// waitForStackIdle. Portainer resets the history at the start of every
// deployment and every call this guards starts one, so an Error left over from
// an earlier deployment (issue #147) cannot survive it; the comparison only
// has to allow for the history's one-second resolution, which lets a quick
// redeploy start in the same second as the deployment before it. Versions
// before 2.45 keep no history and are never reported here.
func waitForStackRedeployed(ctx context.Context, client *APIClient, stackID string, before int64) error {
	stack, err := waitForStackSettled(ctx, client, stackID, false)
	if err != nil {
		return err
	}
	start := stack.deploymentStart()
	if stack.Status != stackStatusError || start == 0 || start < before {
		return nil
	}
	if msg := stack.failureMessage(); msg != "" {
		return fmt.Errorf("redeploying stack %s failed in Portainer: %s", stackID, msg)
	}
	return fmt.Errorf("redeploying stack %s failed in Portainer (status %d); check the stack's logs in Portainer", stackID, stack.Status)
}

// stackDeployState is the part of a stack the deployment waits look at.
type stackDeployState struct {
	Status int `json:"Status"`
	// DeploymentStatus is the history of the current deployment, oldest first
	// (Portainer 2.45 and later).
	DeploymentStatus []struct {
		Status  int    `json:"Status"`
		Time    int64  `json:"Time"`
		Message string `json:"Message"`
	} `json:"DeploymentStatus"`
}

func (s stackDeployState) deploymentStart() int64 {
	if len(s.DeploymentStatus) == 0 {
		return 0
	}
	return s.DeploymentStatus[0].Time
}

func (s stackDeployState) failureMessage() string {
	for i := len(s.DeploymentStatus) - 1; i >= 0; i-- {
		if entry := s.DeploymentStatus[i]; entry.Status == stackStatusError && entry.Message != "" {
			return entry.Message
		}
	}
	return ""
}

// waitForStackSettled blocks until Portainer reports that the stack is no
// longer being deployed. Callers reach it through the two wrappers above
// rather than directly, because whether a failed deployment is an error
// depends entirely on whose deployment it was.
//
// Portainer 2.45 made stack deployment asynchronous: POST /stacks/create/...
// returns as soon as the deploy job is queued, while the stack stays in status
// 3 (Deploying) until docker compose up actually finishes. Every mutating call
// in that window is rejected with 409 "Stack deployment is already in
// progress", which is why creating a standalone or swarm stack failed on every
// apply even though the deployment itself succeeded (issue #145). On 2.39 both
// create and update were synchronous and no lock existed, which is why the same
// configuration worked there.
//
// Waiting on the stack's status rather than simply retrying the mutation
// matters for a reason beyond politeness: a deployment that FAILS also releases
// the lock, so a blind retry would eventually succeed and report a healthy
// apply for a broken stack.
//
// Portainer versions older than 2.45 never report status 3, so this returns
// after the first poll and behaves exactly as before.
func waitForStackSettled(ctx context.Context, client *APIClient, stackID string, failOnFailedDeployment bool) (stackDeployState, error) {
	url := fmt.Sprintf("%s/stacks/%s", client.Endpoint, stackID)
	timedOut := func(cause error) error {
		return fmt.Errorf("gave up waiting for stack %s to finish deploying (raise the resource's create/update timeout if the deployment is simply slow): %w", stackID, cause)
	}

	for {
		if err := ctx.Err(); err != nil {
			return stackDeployState{}, timedOut(err)
		}

		var stack stackDeployState
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &stack); err != nil {
			// The deadline can expire during the request as easily as between
			// two polls; either way it is a timeout, not a broken stack.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return stack, timedOut(ctxErr)
			}
			return stack, fmt.Errorf("failed to read stack %s while waiting for its deployment to settle: %w", stackID, err)
		}

		switch stack.Status {
		case stackStatusDeploying:
			// Still deploying — fall through to the wait below.
		case stackStatusError:
			if !failOnFailedDeployment {
				// A deployment that failed earlier still holds no lock, so the
				// mutation about to be sent can go ahead - and is what fixes
				// the stack.
				return stack, nil
			}
			if msg := stack.failureMessage(); msg != "" {
				return stack, fmt.Errorf("stack %s was deployed but Portainer reports its deployment failed: %s", stackID, msg)
			}
			return stack, fmt.Errorf("stack %s was deployed but Portainer reports its deployment failed (status %d); check the stack's logs in Portainer", stackID, stack.Status)
		default:
			return stack, nil
		}

		select {
		case <-ctx.Done():
			return stack, timedOut(ctx.Err())
		case <-time.After(stackSettlePollInterval):
		}
	}
}

// isStackDeploymentConflict reports whether a Portainer response is the
// deployment lock rejecting a mutating call. The status code alone is not
// enough: 409 is how Portainer answers other conflicts too, and retrying those
// would just loop until the timeout.
func isStackDeploymentConflict(statusCode int, body string) bool {
	return statusCode == http.StatusConflict && strings.Contains(body, "already in progress")
}

// setStackActive starts or stops a stack via the Portainer stack start/stop
// endpoints. The Portainer API has no way to create a stack in the stopped
// state, so callers that want active = false must deploy the stack first and
// then call this with active = false to stop it.
func setStackActive(ctx context.Context, client *APIClient, stackID string, endpointID int, active bool) error {
	action := "stop"
	if active {
		action = "start"
	}

	// Stopping a stack Portainer is still deploying is rejected by the same
	// deployment lock as any other mutation (issue #145), and this runs right
	// after a create or a redeploy — exactly when the lock is held.
	//
	// Every caller reaches this after a deployment of ours: the create at the
	// end of Create, and enforceStackActive after an update or a git redeploy.
	// So a failure here is ours to report, and waiting the failure-aware way is
	// what stops an active = false apply from reporting success for a stack
	// that never came up. Recovering a stack that failed EARLIER is not this
	// function's job — that already happened in the waiter before the update
	// request went out (issue #147).
	if err := waitForStackDeployment(ctx, client, stackID); err != nil {
		return err
	}

	actionURL := fmt.Sprintf("%s/stacks/%s/%s?endpointId=%d", client.Endpoint, stackID, action, endpointID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, actionURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", action, err)
	}
	if err := setAuthHeader(req, client); err != nil {
		return err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to %s stack: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s stack: %s", action, string(body))
	}
	return nil
}

// enforceStackActive applies the desired running state as the final step of an
// update, after any (re)deploy has run. Portainer's update/redeploy endpoints
// always bring the stack back up, so a stack configured active = false must be
// stopped here — regardless of whether "active" itself changed, since a redeploy
// triggered by any other attribute change would otherwise silently restart a
// stack meant to stay stopped (issue #139). active = true needs no action: the
// (re)deploy already left the stack running.
func enforceStackActive(ctx context.Context, client *APIClient, d *dkBag, endpointID int) error {
	if dkBool(d, "active") {
		return nil
	}
	return setStackActive(ctx, client, d.ID(), endpointID, false)
}

func stackUpdate(ctx context.Context, d *dkBag, client *APIClient) error {
	stackID := d.ID()
	endpointID := dkInt(d, "endpoint_id")
	method := dkString(d, "method")

	// NOTE: the desired running state is enforced at the END of this function
	// (see enforceStackActive), not here. Portainer's stack update/redeploy
	// endpoints always bring the stack back up, so stopping before the redeploy
	// would be undone by it — that was the root cause of active=false never
	// persisting on repository stacks (issue #139).

	if method == "file" {
		path := dkString(d, "stack_file_path")
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read stack file for update: %w", err)
		}
		if err := d.Set("stack_file_content", string(content)); err != nil {
			return err
		}
	}

	// ---------------- REPOSITORY STACK ----------------
	if method == "repository" {
		payload := map[string]interface{}{
			"supportRelativePath":       dkBool(d, "support_relative_path"),
			"env":                       stackEnvPayload(d),
			"prune":                     dkBool(d, "prune"),
			"pullImage":                 dkBool(d, "pull_image"),
			"repositoryAuthentication":  dkBool(d, "git_repository_authentication"),
			"repositoryUsername":        dkString(d, "repository_username"),
			"repositoryPassword":        dkString(d, "repository_password"),
			"repositoryReferenceName":   dkString(d, "repository_reference_name"),
			"repositoryGitCredentialID": dkInt(d, "repository_git_credential_id"),
			"sourceID":                  dkInt(d, "source_id"),
			"tlsskipVerify":             dkBool(d, "tlsskip_verify"),
			"additionalFiles":           dkStringSlice(dkList(d, "additional_files")),
			"registries":                dkIntSlice(dkList(d, "registries")),
		}

		if v, ok := d.GetOk("filesystem_path"); ok {
			payload["filesystemPath"] = v.(string)
		}
		addStackKubernetesOptions(d, payload, "kubernetesPrune", "kubernetesForceApply")
		addStackAtomic(d, payload)
		// Portainer 2.42 and later move the stack to another compose file in
		// place. An older version ignores the field, which the response check
		// below turns into an error instead of a plan that never settles.
		configFilePath := dkString(d, "file_path_in_repository")
		if configFilePath != "" {
			payload["configFilePath"] = configFilePath
		}

		webhookID := ""
		if dkBool(d, "stack_webhook") {
			webhookID = dkString(d, "webhook_id")
			if webhookID == "" {
				webhookID = uuid.New().String()
			}

			autoUpdate := map[string]interface{}{
				"ForcePullImage": dkBool(d, "pull_image"),
				"ForceUpdate":    dkBool(d, "force_update"),
				"Interval":       dkString(d, "update_interval"),
				"Webhook":        webhookID,
			}
			payload["AutoUpdate"] = autoUpdate
		} else if v, ok := d.GetOk("update_interval"); ok && v.(string) != "" {
			payload["AutoUpdate"] = map[string]interface{}{
				"ForcePullImage": dkBool(d, "pull_image"),
				"ForceUpdate":    dkBool(d, "force_update"),
				"Interval":       v.(string),
			}
		}

		// Always update git settings via POST /stacks/{id}/git
		// This ensures autoUpdate interval changes are applied
		if err := d.Set("webhook_id", webhookID); err != nil {
			return err
		}
		if webhookID != "" {
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal git update payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s/git?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build git update request: %w", err)
		}
		if err := setAuthHeader(req, client); err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to perform git update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update git stack settings: %s", string(body))
		}
		var updated struct {
			GitConfig *struct {
				ConfigFilePath string `json:"ConfigFilePath"`
			} `json:"GitConfig"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&updated); err == nil && updated.GitConfig != nil &&
			configFilePath != "" && updated.GitConfig.ConfigFilePath != configFilePath {
			//nolint:staticcheck // ST1005: the message users have always seen, kept as it was
			return fmt.Errorf("Portainer kept the stack on %q instead of %q: this Portainer version cannot change file_path_in_repository in place (2.42 or later can). Replace the stack instead, e.g. terraform apply -replace=<address>.", updated.GitConfig.ConfigFilePath, configFilePath)
		}

		redeployPayload := map[string]interface{}{
			"env":                       stackEnvPayload(d),
			"prune":                     dkBool(d, "prune"),
			"pullImage":                 dkBool(d, "pull_image"),
			"repositoryAuthentication":  dkBool(d, "git_repository_authentication"),
			"repositoryUsername":        dkString(d, "repository_username"),
			"repositoryPassword":        dkString(d, "repository_password"),
			"repositoryReferenceName":   dkString(d, "repository_reference_name"),
			"repositoryGitCredentialID": dkInt(d, "repository_git_credential_id"),
			"sourceID":                  dkInt(d, "source_id"),
			"stackName":                 dkString(d, "name"),
			"additionalFiles":           dkStringSlice(dkList(d, "additional_files")),
			"registries":                dkIntSlice(dkList(d, "registries")),
		}
		addStackRepull(d, redeployPayload)

		redeployBody, err := json.Marshal(redeployPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal git redeploy payload: %w", err)
		}

		// Portainer rejects this while an earlier deployment is still running
		// (issue #145) — a webhook or another apply can leave one in flight.
		// Only the lock is waited out here: a deployment that already failed is
		// exactly what this call replaces (issue #147).
		before, err := waitForStackIdle(ctx, client, stackID)
		if err != nil {
			return err
		}

		redeployURL := fmt.Sprintf("%s/stacks/%s/git/redeploy?endpointId=%d", client.Endpoint, stackID, endpointID)
		reqRedeploy, err := http.NewRequestWithContext(ctx, http.MethodPut, redeployURL, bytes.NewBuffer(redeployBody))
		if err != nil {
			return fmt.Errorf("failed to build git redeploy request: %w", err)
		}
		if err := setAuthHeader(reqRedeploy, client); err != nil {
			return err
		}
		reqRedeploy.Header.Set("Content-Type", "application/json")

		respRedeploy, err := client.HTTPClient.Do(reqRedeploy)
		if err != nil {
			return fmt.Errorf("failed to perform git redeploy request: %w", err)
		}
		defer respRedeploy.Body.Close()

		if respRedeploy.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(respRedeploy.Body)
			return fmt.Errorf("failed to redeploy git stack: %s", string(data))
		}

		// Portainer 2.45 redeploys asynchronously. Waiting here keeps Read from
		// seeing the stack mid-deployment, which showed as active = false in
		// the next plan, and reports a redeploy that failed.
		if err := waitForStackRedeployed(ctx, client, stackID, before); err != nil {
			return err
		}

		// Apply the desired running state AFTER the git redeploy (which restarts
		// the stack), so active = false actually persists for repository stacks.
		if err := enforceStackActive(ctx, client, d, endpointID); err != nil {
			return err
		}

		return stackRead(ctx, d, client)
	}

	if err := updateStackAccessControl(ctx, d, client, stackID); err != nil {
		return fmt.Errorf("failed to update stack access control: %w", err)
	}

	// ---------------- NON-REPOSITORY STACKS ----------------
	// before marks the deployment that preceded this update, so that only a
	// redeploy started here is reported as failed.
	var before int64
	if method != "repository" {
		payload := map[string]interface{}{
			"env":              stackEnvPayload(d),
			"stackFileContent": dkString(d, "stack_file_content"),
			"prune":            dkBool(d, "prune"),
			"pullImage":        dkBool(d, "pull_image"),
		}
		addStackRepull(d, payload)

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal standard update payload: %w", err)
		}

		// Portainer rejects this while an earlier deployment is still running
		// (issue #145) — a webhook or another apply can leave one in flight.
		// Only the lock is waited out here: a deployment that already failed is
		// exactly what this call replaces (issue #147).
		before, err = waitForStackIdle(ctx, client, stackID)
		if err != nil {
			return err
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build standard update request: %w", err)
		}
		if err := setAuthHeader(req, client); err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to perform standard update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update stack: %s", string(data))
		}
	}

	if dkBool(d, "stack_webhook") && method != "repository" {
		webhookToken := dkString(d, "webhook_id")
		if webhookToken == "" {
			webhookToken = uuid.New().String()
		}

		payload := map[string]interface{}{
			"env":              stackEnvPayload(d),
			"stackFileContent": dkString(d, "stack_file_content"),
			"prune":            dkBool(d, "prune"),
			"pullImage":        dkBool(d, "pull_image"),
			"webhook":          webhookToken,
			"registries":       dkIntSlice(dkList(d, "registries")),
		}
		addStackRepull(d, payload)

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal webhook update payload: %w", err)
		}

		// Portainer rejects this while an earlier deployment is still running
		// (issue #145) — a webhook or another apply can leave one in flight.
		// Only the lock is waited out here: a deployment that already failed is
		// exactly what this call replaces (issue #147).
		if err := waitForStackLockReleased(ctx, client, d.ID()); err != nil {
			return err
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.ID(), endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build webhook update request: %w", err)
		}
		if err := setAuthHeader(req, client); err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to perform webhook update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update stack webhook, status %d: %s", resp.StatusCode, string(data))
		}

		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		if err := d.SetFields(map[string]interface{}{
			"webhook_id":  webhookToken,
			"webhook_url": webhookURL,
		}); err != nil {
			return err
		}
	}

	// Same as for repository stacks above: wait for the asynchronous redeploy
	// and report it if it failed.
	if err := waitForStackRedeployed(ctx, client, stackID, before); err != nil {
		return err
	}

	// Apply the desired running state AFTER the stack (re)deploy above, so
	// active = false persists instead of being undone by the update.
	if err := enforceStackActive(ctx, client, d, endpointID); err != nil {
		return err
	}

	return stackRead(ctx, d, client)
}

func flattenEnvList(envList []interface{}) []map[string]string {
	out := make([]map[string]string, 0, len(envList))
	for _, v := range envList {
		item := v.(map[string]interface{})
		out = append(out, map[string]string{
			"name":  item["name"].(string),
			"value": item["value"].(string),
		})
	}
	return out
}

// stackEnvPayload returns env followed by env_wo. The env_wo values are
// write-only, so they are taken from the raw configuration, which carries them
// on every apply. Portainer replaces the whole list on each deploy, so they
// have to be sent every time, not only when env_wo_version changes.
func stackEnvPayload(d *dkBag) []map[string]string {
	out := flattenEnvList(dkList(d, "env"))

	raw := d.ConfigValue("env_wo")
	if !raw.IsKnown() || raw.IsNull() {
		return out
	}
	var items []tftypes.Value
	if err := raw.As(&items); err != nil {
		return out
	}
	for _, item := range items {
		attrs := dkObjectAttrs(item)
		name, value := attrs["name"], attrs["value"]
		if name.Type() == nil || value.Type() == nil || !name.IsKnown() || name.IsNull() || !value.IsKnown() || value.IsNull() {
			continue
		}
		var n, v string
		_ = name.As(&n)
		_ = value.As(&v)
		out = append(out, map[string]string{
			"name":  n,
			"value": v,
		})
	}
	return out
}

// envNames returns the non-empty variable names of an env or env_wo list.
func envNames(list []interface{}) []string {
	names := make([]string, 0, len(list))
	for _, v := range list {
		if item, ok := v.(map[string]interface{}); ok {
			if name, _ := item["name"].(string); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// addStackAtomic adds Atomic (Helm --rollback-on-failure, Business Edition
// 3.1) when the configuration sets atomic. Left out, it is not sent:
// Portainer keeps its recorded choice and an older server never sees it.
func addStackAtomic(d *dkBag, payload map[string]interface{}) {
	if v := d.ConfigBool("atomic"); v != nil && dkString(d, "helm_chart_path") != "" {
		payload["atomic"] = *v
	}
}

// stackAtomicTracked reports whether state or plan holds atomic, which is
// then read back; null means the configuration leaves it to Portainer.
func stackAtomicTracked(d *dkBag) bool {
	v, ok := d.vals["atomic"]
	return ok && v != nil
}

// addStackKubernetesOptions adds the Kubernetes apply policies the
// configuration sets to payload under pruneKey and forceApplyKey. One that is
// left out is not sent: Portainer then keeps its recorded choice (or its
// default on create), and a Portainer older than 3.0 never sees the field.
// addStackRepull adds RepullImageAndRedeploy, which replaces the pullImage
// flag Portainer deprecated in 2.36, when pull_image is on. pullImage stays
// for older servers; false is not sent, the same as the deprecated flag left
// at its default.
func addStackRepull(d *dkBag, payload map[string]interface{}) {
	if dkBool(d, "pull_image") {
		payload["repullImageAndRedeploy"] = true
	}
}

func addStackKubernetesOptions(d *dkBag, payload map[string]interface{}, pruneKey, forceApplyKey string) {
	if dkString(d, "deployment_type") != "kubernetes" {
		return
	}
	if v := d.ConfigBool("kubernetes_prune"); v != nil {
		payload[pruneKey] = *v
	}
	if v := d.ConfigBool("kubernetes_force_apply"); v != nil {
		payload[forceApplyKey] = *v
	}
}

// --------------------- STANDALONE ----------------------

func createStackStandaloneString(ctx context.Context, d *dkBag, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             dkString(d, "name"),
		"stackFileContent": dkString(d, "stack_file_content"),
		"env":              stackEnvPayload(d),
		"fromAppTemplate":  false,
		"registries":       dkIntSlice(dkList(d, "registries")),
	}
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/standalone/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

func createStackStandaloneRepo(ctx context.Context, d *dkBag, client *APIClient) error {
	repoURL := dkString(d, "repository_url")
	repoUser := dkString(d, "repository_username")
	repoPass := dkString(d, "repository_password")

	if dkInt(d, "repository_wo_version") != 0 {
		if d.Configured("repository_url_wo") {
			repoURL = d.ConfigString("repository_url_wo")
		}
		if d.Configured("repository_username_wo") {
			repoUser = d.ConfigString("repository_username_wo")
		}
		if d.Configured("repository_password_wo") {
			repoPass = d.ConfigString("repository_password_wo")
		}
	}

	composeFile := dkString(d, "file_path_in_repository")
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	payload := map[string]interface{}{
		"name":                      dkString(d, "name"),
		"composeFile":               composeFile,
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   dkString(d, "repository_reference_name"),
		"repositoryAuthentication":  dkBool(d, "git_repository_authentication"),
		"repositoryGitCredentialID": dkInt(d, "repository_git_credential_id"),
		"sourceID":                  dkInt(d, "source_id"),
		"supportRelativePath":       dkBool(d, "support_relative_path"),
		"env":                       stackEnvPayload(d),
		"fromAppTemplate":           false,
		"tlsskipVerify":             dkBool(d, "tlsskip_verify"),
		"additionalFiles":           dkStringSlice(dkList(d, "additional_files")),
	}

	if v, ok := d.GetOk("filesystem_path"); ok {
		payload["filesystemPath"] = v.(string)
	}

	stackWebhook := dkBool(d, "stack_webhook")
	if stackWebhook || dkString(d, "update_interval") != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": dkBool(d, "pull_image"),
			"forceUpdate":    dkBool(d, "force_update"),
			"interval":       dkString(d, "update_interval"),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = dkIntSlice(dkList(d, "registries"))
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/standalone/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

// --------------------- SWARM ----------------------

func createStackSwarmString(ctx context.Context, d *dkBag, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             dkString(d, "name"),
		"stackFileContent": dkString(d, "stack_file_content"),
		"env":              stackEnvPayload(d),
		"fromAppTemplate":  false,
		"swarmID":          dkString(d, "swarm_id"),
		"registries":       dkIntSlice(dkList(d, "registries")),
	}
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/swarm/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

func createStackSwarmRepo(ctx context.Context, d *dkBag, client *APIClient) error {
	repoURL := dkString(d, "repository_url")
	repoUser := dkString(d, "repository_username")
	repoPass := dkString(d, "repository_password")

	if dkInt(d, "repository_wo_version") != 0 {
		if d.Configured("repository_url_wo") {
			repoURL = d.ConfigString("repository_url_wo")
		}
		if d.Configured("repository_username_wo") {
			repoUser = d.ConfigString("repository_username_wo")
		}
		if d.Configured("repository_password_wo") {
			repoPass = d.ConfigString("repository_password_wo")
		}
	}

	composeFile := dkString(d, "file_path_in_repository")
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	payload := map[string]interface{}{
		"name":                      dkString(d, "name"),
		"composeFile":               composeFile,
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   dkString(d, "repository_reference_name"),
		"repositoryAuthentication":  dkBool(d, "git_repository_authentication"),
		"repositoryGitCredentialID": dkInt(d, "repository_git_credential_id"),
		"sourceID":                  dkInt(d, "source_id"),
		"supportRelativePath":       dkBool(d, "support_relative_path"),
		"env":                       stackEnvPayload(d),
		"fromAppTemplate":           false,
		"tlsskipVerify":             dkBool(d, "tlsskip_verify"),
		"swarmID":                   dkString(d, "swarm_id"),
		"additionalFiles":           dkStringSlice(dkList(d, "additional_files")),
	}

	if v, ok := d.GetOk("filesystem_path"); ok {
		payload["filesystemPath"] = v.(string)
	}

	stackWebhook := dkBool(d, "stack_webhook")
	if stackWebhook || dkString(d, "update_interval") != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": dkBool(d, "pull_image"),
			"forceUpdate":    dkBool(d, "force_update"),
			"interval":       dkString(d, "update_interval"),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = dkIntSlice(dkList(d, "registries"))
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/swarm/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))

	if dkBool(d, "prune") {
		log.Printf("[INFO] Performing immediate redeploy with prune=true after stack creation")
		// The redeploy is waited out and a failed one reported; logging it
		// only reported a broken stack as created.
		if err := stackUpdate(ctx, d, client); err != nil {
			return fmt.Errorf("prune redeploy after creating stack %s failed: %s", d.ID(), err.Error())
		}
	}

	return nil
}

// --------------------- KUBERNETES ----------------------

func createStackK8sString(ctx context.Context, d *dkBag, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":        dkString(d, "name"),
		"stackFileContent": dkString(d, "stack_file_content"),
		"namespace":        dkString(d, "namespace"),
		"composeFormat":    dkBool(d, "compose_format"),
		"fromAppTemplate":  false,
		"registries":       dkIntSlice(dkList(d, "registries")),
	}
	addStackKubernetesOptions(d, payload, "prune", "forceApply")
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/kubernetes/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from string: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sRepo(ctx context.Context, d *dkBag, client *APIClient) error {
	repoURL := dkString(d, "repository_url")
	repoUser := dkString(d, "repository_username")
	repoPass := dkString(d, "repository_password")

	if dkInt(d, "repository_wo_version") != 0 {
		if d.Configured("repository_url_wo") {
			repoURL = d.ConfigString("repository_url_wo")
		}
		if d.Configured("repository_username_wo") {
			repoUser = d.ConfigString("repository_username_wo")
		}
		if d.Configured("repository_password_wo") {
			repoPass = d.ConfigString("repository_password_wo")
		}
	}
	helmChartPath := ""
	if v, ok := d.GetOk("helm_chart_path"); ok {
		helmChartPath = v.(string)
	}

	manifestFile := dkString(d, "file_path_in_repository")
	if helmChartPath != "" {
		// When deploying via Helm chart, manifestFile is not needed
		manifestFile = ""
	}

	payload := map[string]interface{}{
		"stackName":                 dkString(d, "name"),
		"manifestFile":              manifestFile,
		"namespace":                 dkString(d, "namespace"),
		"composeFormat":             dkBool(d, "compose_format"),
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   dkString(d, "repository_reference_name"),
		"repositoryAuthentication":  dkBool(d, "git_repository_authentication"),
		"repositoryGitCredentialID": dkInt(d, "repository_git_credential_id"),
		"sourceID":                  dkInt(d, "source_id"),
		"tlsskipVerify":             dkBool(d, "tlsskip_verify"),
		"fromAppTemplate":           false,
		"additionalFiles":           dkStringSlice(dkList(d, "additional_files")),
	}

	if helmChartPath != "" {
		payload["helmChartPath"] = helmChartPath
		if valuesFiles, ok := d.GetOk("additional_helm_values_files"); ok {
			payload["helmValuesFiles"] = dkStringSlice(valuesFiles.([]interface{}))
		}
		addStackAtomic(d, payload)
	}

	stackWebhook := dkBool(d, "stack_webhook")
	if stackWebhook || dkString(d, "update_interval") != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": dkBool(d, "pull_image"),
			"forceUpdate":    dkBool(d, "force_update"),
			"interval":       dkString(d, "update_interval"),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = dkIntSlice(dkList(d, "registries"))
	addStackKubernetesOptions(d, payload, "prune", "forceApply")
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/kubernetes/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sURL(ctx context.Context, d *dkBag, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":     dkString(d, "name"),
		"manifestURL":   dkString(d, "manifest_url"),
		"namespace":     dkString(d, "namespace"),
		"composeFormat": dkBool(d, "compose_format"),
		"registries":    dkIntSlice(dkList(d, "registries")),
	}
	addStackKubernetesOptions(d, payload, "prune", "forceApply")
	endpointID := dkInt(d, "endpoint_id")
	url := fmt.Sprintf("%s/stacks/create/kubernetes/url?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from URL: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetID(strconv.Itoa(result.ID))
	return nil
}

func updateStackAccessControl(ctx context.Context, d *dkBag, client *APIClient, stackID string) error {
	// If ownership is not set, we might default to something or just return
	// But if d.Get("ownership") is set, it might be "", which is default.
	// But "ownership" is computed so default is likely not "".

	// Check if ownership is set in TF config
	ownership := dkString(d, "ownership")
	if ownership == "" {
		// No ownership change requested or managed
		return nil
	}

	// Retrieve the ResourceControlID for this stack
	rcIDString, _, err := lookupResourceControlID(ctx, client, 6, stackID) // 6 = stack
	if err != nil {
		// If resource control doesn't exist, we can't update it.
		// But stacks usually have one created by default.
		return fmt.Errorf("failed to lookup resource control for stack %s: %w", stackID, err)
	}

	rcID := rcIDString

	// Prepare update payload
	payload := map[string]interface{}{}

	switch ownership {
	case "public":
		payload["public"] = true
		payload["administratorsOnly"] = false
		payload["users"] = []int{}
		payload["teams"] = []int{}
	case "administrators":
		payload["public"] = false
		payload["administratorsOnly"] = true
		payload["users"] = []int{}
		payload["teams"] = []int{}
	case "restricted", "private":
		payload["public"] = false
		payload["administratorsOnly"] = false
		// Only set users/teams if restricted
		if v, ok := d.GetOk("authorized_users"); ok {
			payload["users"] = dkIntSlice(v.([]interface{}))
		} else {
			payload["users"] = []int{}
		}

		if v, ok := d.GetOk("authorized_teams"); ok {
			payload["teams"] = dkIntSlice(v.([]interface{}))
		} else {
			payload["teams"] = []int{}
		}
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), payload, nil); err != nil {
		return fmt.Errorf("failed to update resource control %s: %w", rcID, err)
	}

	return nil
}

func readStackAccessControl(ctx context.Context, d *dkBag, client *APIClient, rcID string) error {
	var rc struct {
		AdministratorsOnly bool `json:"AdministratorsOnly"`
		Public             bool `json:"Public"`
		TeamAccesses       []struct {
			TeamID int `json:"TeamId"`
		} `json:"TeamAccesses"`
		UserAccesses []struct {
			UserID int `json:"UserId"`
		} `json:"UserAccesses"`
	}

	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), nil, &rc); err != nil {
		return fmt.Errorf("failed to fetch resource control %s: %w", rcID, err)
	}

	if rc.Public {
		if err := d.Set("ownership", "public"); err != nil {
			return err
		}
	} else if rc.AdministratorsOnly {
		if err := d.Set("ownership", "administrators"); err != nil {
			return err
		}
	} else {
		if err := d.Set("ownership", "restricted"); err != nil {
			return err
		}
	}

	users := []int{}
	for _, u := range rc.UserAccesses {
		users = append(users, u.UserID)
	}
	if err := d.Set("authorized_users", users); err != nil {
		return err
	}

	teams := []int{}
	for _, t := range rc.TeamAccesses {
		teams = append(teams, t.TeamID)
	}
	if err := d.Set("authorized_teams", teams); err != nil {
		return err
	}

	return nil
}
