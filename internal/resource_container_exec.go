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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceContainerExec() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceContainerExecCreate,
		ReadContext:   resourceContainerExecRead,
		DeleteContext: resourceContainerExecDelete,
		UpdateContext: nil,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id":  {Type: schema.TypeInt, Required: true, ForceNew: true, Description: "Identifier of the Portainer endpoint hosting the container or Swarm service."},
			"service_name": {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Name of the container (standalone) or Swarm service in which the command should be executed."},
			"user":         {Type: schema.TypeString, Optional: true, ForceNew: true, Default: "root:root", Description: "User and optional group used to run the command inside the container (defaults to `root:root`)."},
			"command":      {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Shell command to execute inside the target container."},
			"wait":         {Type: schema.TypeInt, Optional: true, ForceNew: true, Default: 0, Description: "Initial delay in seconds before executing the command."},
			"mode":         {Type: schema.TypeString, Optional: true, ForceNew: true, Default: "standalone", Description: "Deployment mode: 'standalone' (default) or 'swarm'"},
			"output":       {Type: schema.TypeString, Computed: true, ForceNew: true, Description: "Standard output captured from the executed command."},
		},
	}
}

func resourceContainerExecCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	mode := d.Get("mode").(string)
	if mode == "swarm" {
		return diag.FromErr(execInSwarm(ctx, d, meta))
	}
	return diag.FromErr(execInStandalone(ctx, d, meta))
}

func execInStandalone(ctx context.Context, d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	container := d.Get("service_name").(string)
	user := d.Get("user").(string)
	command := d.Get("command").(string)
	wait := d.Get("wait").(int)

	timeout := d.Timeout(schema.TimeoutCreate)
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
	d.SetId(execResult.ID)
	return nil
}

func execInSwarm(ctx context.Context, d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	service := d.Get("service_name").(string)
	user := d.Get("user").(string)
	command := d.Get("command").(string)
	wait := d.Get("wait").(int)

	timeout := d.Timeout(schema.TimeoutCreate)
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
	d.SetId(execResult.ID)
	return nil
}

func resourceContainerExecRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil // Stateless
}

func resourceContainerExecDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
