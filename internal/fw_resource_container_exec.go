package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// portainer_container_exec runs a command in a container (or in the task of
// a Swarm service) once, at create. Read and Delete do nothing.

var newContainerExecResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_container_exec",
	schema:     containerExecSchema,
	create:     containerExecCreate,
	read:       containerExecRead,
	delete:     containerExecDelete,
	timeouts:   map[string]time.Duration{"create": 5 * time.Minute},
})

func containerExecSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"command":      schema.StringAttribute{Required: true, Description: "Shell command to execute inside the target container.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"endpoint_id":  schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint hosting the container or Swarm service.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"mode":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("standalone"), Description: "Deployment mode: 'standalone' (default) or 'swarm'", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"output":       schema.StringAttribute{Computed: true, Description: "Standard output captured from the executed command."},
			"service_name": schema.StringAttribute{Required: true, Description: "Name of the container (standalone) or Swarm service in which the command should be executed.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"user":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("root:root"), Description: "User and optional group used to run the command inside the container (defaults to `root:root`).", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"wait":         schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0), Description: "Initial delay in seconds before executing the command.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true}),
		},
	}
}

func containerExecCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	mode := dkString(d, "mode")
	if mode == "swarm" {
		return execInSwarm(ctx, d, client)
	}
	return execInStandalone(ctx, d, client)
}

func execInStandalone(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	container := dkString(d, "service_name")
	user := dkString(d, "user")
	command := dkString(d, "command")
	wait := dkInt(d, "wait")

	timeout := d.Timeout("create", 5*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if wait > 0 {
		if err := sleepCtx(ctx, time.Duration(wait)*time.Second); err != nil {
			return err
		}
	}

	filter := fmt.Sprintf(`{"name":["%s"]}`, container)
	containersURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?filters=%s", client.Endpoint, endpointID, url.QueryEscape(filter))
	containersResp, err := apiGETCtx(ctx, containersURL, client.APIKey, client)
	if err != nil {
		return err
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(containersResp, &containers); err != nil || len(containers) == 0 {
		return fmt.Errorf("no container found with name %s", container)
	}

	containerID, _ := containers[0]["Id"].(string)
	if containerID == "" {
		return fmt.Errorf("container %s has no ID in the Docker response", container)
	}

	commandSplit := strings.Fields(command)
	execBody := map[string]interface{}{
		"User":         user,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Cmd":          commandSplit,
	}

	execReqBody, _ := json.Marshal(execBody)
	execURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/%s/exec", client.Endpoint, endpointID, containerID)
	execReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, execURL, bytes.NewBuffer(execReqBody))
	if err := setAuthHeader(execReq, client); err != nil {
		return err
	}
	execReq.Header.Set("Content-Type", "application/json")
	execResp, err := client.HTTPClient.Do(execReq)
	if err != nil {
		return err
	}
	defer execResp.Body.Close()

	var execResult struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(execResp.Body).Decode(&execResult); err != nil {
		return fmt.Errorf("failed to decode exec result: %w", err)
	}

	startURL := fmt.Sprintf("%s/endpoints/%d/docker/exec/%s/start", client.Endpoint, endpointID, execResult.ID)
	startBody := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}
	startReqBody, _ := json.Marshal(startBody)
	startReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewBuffer(startReqBody))
	if err := setAuthHeader(startReq, client); err != nil {
		return err
	}
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.HTTPClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	output, _ := io.ReadAll(startResp.Body)
	if err := d.Set("output", string(output)); err != nil {
		return err
	}
	d.SetID(execResult.ID)
	return nil
}

func execInSwarm(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	service := dkString(d, "service_name")
	user := dkString(d, "user")
	command := dkString(d, "command")
	wait := dkInt(d, "wait")

	timeout := d.Timeout("create", 5*time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if wait > 0 {
		if err := sleepCtx(ctx, time.Duration(wait)*time.Second); err != nil {
			return err
		}
	}

	filter := fmt.Sprintf(`{"service":{"%s":true},"desired-state":{"running":true}}`, service)
	encodedFilter := url.QueryEscape(filter)
	tasksURL := fmt.Sprintf("%s/endpoints/%d/docker/tasks?filters=%s", client.Endpoint, endpointID, encodedFilter)
	tasksResp, err := apiGETCtx(ctx, tasksURL, client.APIKey, client)
	if err != nil {
		return err
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(tasksResp, &tasks); err != nil || len(tasks) == 0 {
		return fmt.Errorf("failed to parse tasks or no tasks found")
	}

	// A task that is not scheduled yet has no node or container, which used to
	// crash the provider on the type assertions below.
	nodeID, _ := tasks[0]["NodeID"].(string)
	containerID, _ := mustMap(mustMap(tasks[0]["Status"])["ContainerStatus"])["ContainerID"].(string)
	if nodeID == "" || containerID == "" {
		return fmt.Errorf("the task of service %s has no container yet; retry once it is running", service)
	}
	nodeResp, err := apiGETCtx(ctx, fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, nodeID), client.APIKey, client)
	if err != nil {
		return err
	}
	var node map[string]interface{}
	if err := json.Unmarshal(nodeResp, &node); err != nil {
		return fmt.Errorf("failed to decode node info: %w", err)
	}
	hostname, _ := mustMap(node["Description"])["Hostname"].(string)
	if hostname == "" {
		return fmt.Errorf("swarm node %s reports no hostname, so the exec cannot be routed to it", nodeID)
	}

	commandSplit := strings.Fields(command)
	execBody := map[string]interface{}{
		"User":         user,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          true,
		"Cmd":          commandSplit,
	}

	execReqBody, _ := json.Marshal(execBody)
	execURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/%s/exec", client.Endpoint, endpointID, containerID)
	execReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, execURL, bytes.NewBuffer(execReqBody))
	if err := setAuthHeader(execReq, client); err != nil {
		return err
	}
	execReq.Header.Set("X-PortainerAgent-Target", hostname)
	execReq.Header.Set("Content-Type", "application/json")
	execResp, err := client.HTTPClient.Do(execReq)
	if err != nil {
		return err
	}
	defer execResp.Body.Close()

	var execResult struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(execResp.Body).Decode(&execResult); err != nil {
		return fmt.Errorf("failed to decode exec result: %w", err)
	}

	startURL := fmt.Sprintf("%s/endpoints/%d/docker/exec/%s/start", client.Endpoint, endpointID, execResult.ID)
	startBody := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}
	startReqBody, _ := json.Marshal(startBody)
	startReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewBuffer(startReqBody))
	if err := setAuthHeader(startReq, client); err != nil {
		return err
	}
	startReq.Header.Set("X-PortainerAgent-Target", hostname)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := client.HTTPClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	output, _ := io.ReadAll(startResp.Body)
	if err := d.Set("output", string(output)); err != nil {
		return err
	}
	d.SetID(execResult.ID)
	return nil
}

func containerExecRead(ctx context.Context, d *dkBag, client *APIClient) error {
	return nil // Stateless
}

func containerExecDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	d.SetID("")
	return nil
}
