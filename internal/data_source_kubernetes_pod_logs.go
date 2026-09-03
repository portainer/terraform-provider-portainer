package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceKubernetesPodLogs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPodLogsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment the pod runs in.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace the pod lives in.",
			},
			"pod_name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the pod to read logs from.",
			},
			"container": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Container to read logs from. Required when the pod runs more than one container.",
			},
			"tail_lines": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(1),
				Description:  "Number of lines to return from the end of the log. Leave unset to return the whole available log, which can be large.",
			},
			"since_seconds": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(1),
				Description:  "Only return log lines newer than this many seconds.",
			},
			"timestamps": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to prefix every line with an RFC3339 timestamp.",
			},
			"previous": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to read the log of the previous terminated instance of the container, which is how a crash-looping container's last output is retrieved.",
			},
			// Computed attributes
			"logs": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Log output returned by the pod, as plain text.",
			},
			"line_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of non-empty lines in `logs`.",
			},
		},
	}
}

func dataSourceKubernetesPodLogsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	podName := d.Get("pod_name").(string)

	// The `follow` parameter is deliberately not exposed: it keeps the response
	// open until the client disconnects, which would hang a plan or apply.
	q := url.Values{}
	if v, ok := d.GetOk("container"); ok {
		q.Set("container", v.(string))
	}
	if v, ok := d.GetOk("tail_lines"); ok {
		q.Set("tailLines", strconv.Itoa(v.(int)))
	}
	if v, ok := d.GetOk("since_seconds"); ok {
		q.Set("sinceSeconds", strconv.Itoa(v.(int)))
	}
	if d.Get("timestamps").(bool) {
		q.Set("timestamps", "true")
	}
	if d.Get("previous").(bool) {
		q.Set("previous", "true")
	}

	logURL := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/pods/%s/log", client.Endpoint, envID, namespace, podName)
	if len(q) > 0 {
		logURL += "?" + q.Encode()
	}

	// The endpoint answers text/plain, so this goes through the raw byte helper
	// rather than doJSON.
	body, status, err := apiGETWithCodeCtx(ctx, logURL, client.APIKey, client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read logs of pod %q in namespace %q: %w", podName, namespace, err))
	}
	if status >= http.StatusBadRequest {
		return diag.FromErr(fmt.Errorf("failed to read logs of pod %q in namespace %q: status %d: %s", podName, namespace, status, strings.TrimSpace(string(body))))
	}

	logs := string(body)
	lineCount := 0
	for _, line := range strings.Split(logs, "\n") {
		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}

	if err := setFields(d, map[string]interface{}{
		"logs":       logs,
		"line_count": lineCount,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/%s/log", envID, namespace, podName))
	return nil
}
