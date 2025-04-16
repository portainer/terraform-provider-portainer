package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type execEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func resourceContainerExec() *schema.Resource {
	return &schema.Resource{
		Create: resourceContainerExecCreate,
		Read:   resourceContainerExecRead,
		Delete: resourceContainerExecDelete,
		Update: nil,
		Schema: map[string]*schema.Schema{
			"endpoint_id":  {Type: schema.TypeInt, Required: true, ForceNew: true},
			"service_name": {Type: schema.TypeString, Required: true, ForceNew: true},
			"user":         {Type: schema.TypeString, Optional: true, ForceNew: true, Default: "root:root"},
			"command":      {Type: schema.TypeString, Required: true, ForceNew: true},
			"wait":         {Type: schema.TypeInt, Optional: true, ForceNew: true, Default: 0},
			"mode":         {Type: schema.TypeString, Optional: true, ForceNew: true, Default: "standalone", Description: "Deployment mode: 'standalone' (default) or 'swarm'"},
			"output":       {Type: schema.TypeString, Computed: true, ForceNew: true},
		},
	}
}

func resourceContainerExecCreate(d *schema.ResourceData, meta interface{}) error {
	mode := d.Get("mode").(string)
	if mode == "swarm" {
		return execInSwarm(d, meta)
	}
	return execInStandalone(d, meta)
}

func execInStandalone(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	container := d.Get("service_name").(string)
	user := d.Get("user").(string)
	command := d.Get("command").(string)
	wait := d.Get("wait").(int)

	if wait > 0 {
		time.Sleep(time.Duration(wait) * time.Second)
	}

	filter := fmt.Sprintf(`{"name":["%s"]}`, container)
	containersURL := fmt.Sprintf("%s/endpoints/%d/docker/containers/json?filters=%s", client.Endpoint, endpointID, url.QueryEscape(filter))
	containersResp, err := apiGET(containersURL, client.APIKey)
	if err != nil {
		return err
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(containersResp, &containers); err != nil || len(containers) == 0 {
		return fmt.Errorf("no container found with name %s", container)
	}

	containerID := containers[0]["Id"].(string)

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
	execReq, _ := http.NewRequest("POST", execURL, bytes.NewBuffer(execReqBody))
	execReq.Header.Set("X-API-Key", client.APIKey)
	execReq.Header.Set("Content-Type", "application/json")
	execResp, err := http.DefaultClient.Do(execReq)
	if err != nil {
		return err
	}
	defer execResp.Body.Close()

	var execResult struct {
		ID string `json:"Id"`
	}
	json.NewDecoder(execResp.Body).Decode(&execResult)

	startURL := fmt.Sprintf("%s/endpoints/%d/docker/exec/%s/start", client.Endpoint, endpointID, execResult.ID)
	startBody := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}
	startReqBody, _ := json.Marshal(startBody)
	startReq, _ := http.NewRequest("POST", startURL, bytes.NewBuffer(startReqBody))
	startReq.Header.Set("X-API-Key", client.APIKey)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := http.DefaultClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	output, _ := io.ReadAll(startResp.Body)
	d.Set("output", string(output))
	d.SetId(execResult.ID)
	return nil
}

func execInSwarm(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	service := d.Get("service_name").(string)
	user := d.Get("user").(string)
	command := d.Get("command").(string)
	wait := d.Get("wait").(int)

	if wait > 0 {
		time.Sleep(time.Duration(wait) * time.Second)
	}

	filter := fmt.Sprintf(`{"service":{"%s":true},"desired-state":{"running":true}}`, service)
	encodedFilter := url.QueryEscape(filter)
	tasksURL := fmt.Sprintf("%s/endpoints/%d/docker/tasks?filters=%s", client.Endpoint, endpointID, encodedFilter)
	tasksResp, err := apiGET(tasksURL, client.APIKey)
	if err != nil {
		return err
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(tasksResp, &tasks); err != nil || len(tasks) == 0 {
		return fmt.Errorf("failed to parse tasks or no tasks found")
	}

	nodeID := tasks[0]["NodeID"].(string)
	nodeResp, err := apiGET(fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, nodeID), client.APIKey)
	if err != nil {
		return err
	}
	var node map[string]interface{}
	json.Unmarshal(nodeResp, &node)
	hostname := node["Description"].(map[string]interface{})["Hostname"].(string)

	containerID := tasks[0]["Status"].(map[string]interface{})["ContainerStatus"].(map[string]interface{})["ContainerID"].(string)

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
	execReq, _ := http.NewRequest("POST", execURL, bytes.NewBuffer(execReqBody))
	execReq.Header.Set("X-API-Key", client.APIKey)
	execReq.Header.Set("X-PortainerAgent-Target", hostname)
	execReq.Header.Set("Content-Type", "application/json")
	execResp, err := http.DefaultClient.Do(execReq)
	if err != nil {
		return err
	}
	defer execResp.Body.Close()

	var execResult struct {
		ID string `json:"Id"`
	}
	json.NewDecoder(execResp.Body).Decode(&execResult)

	startURL := fmt.Sprintf("%s/endpoints/%d/docker/exec/%s/start", client.Endpoint, endpointID, execResult.ID)
	startBody := map[string]interface{}{
		"Detach": false,
		"Tty":    false,
	}
	startReqBody, _ := json.Marshal(startBody)
	startReq, _ := http.NewRequest("POST", startURL, bytes.NewBuffer(startReqBody))
	startReq.Header.Set("X-API-Key", client.APIKey)
	startReq.Header.Set("X-PortainerAgent-Target", hostname)
	startReq.Header.Set("Content-Type", "application/json")
	startResp, err := http.DefaultClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	output, _ := io.ReadAll(startResp.Body)
	d.Set("output", string(output))
	d.SetId(execResult.ID)
	return nil
}

func resourceContainerExecRead(d *schema.ResourceData, meta interface{}) error {
	return nil // Stateless
}

func resourceContainerExecDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

func apiGET(url string, apiKey string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-API-Key", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
