package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesEventsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_events", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSOptionalString("Namespace to read events from. Leave unset to read events across every namespace the API token can access."),
			"resource_id":    k8sDSOptionalString("Only return events for the object with this UID. Cluster-wide queries only."),
			// Computed attributes
			"events": k8sDSObjectList("Events matching the query, as Kubernetes reports them. Events are short-lived — Kubernetes discards them after about an hour by default.",
				k8sDSField{"type", types.StringType, "Event type, `Normal` or `Warning`."},
				k8sDSField{"reason", types.StringType, "Short machine-readable reason, such as `FailedScheduling`."},
				k8sDSField{"message", types.StringType, "Human-readable description of what happened."},
				k8sDSField{"count", types.Int64Type, "How many times this event has occurred."},
				k8sDSField{"namespace", types.StringType, "Namespace the event belongs to."},
				k8sDSField{"kind", types.StringType, "Kind of the object the event is about."},
				k8sDSField{"name", types.StringType, "Name of the object the event is about."},
				k8sDSField{"uid", types.StringType, "UID of the object the event is about."},
				k8sDSField{"first_timestamp", types.StringType, "When the event was first recorded."},
				k8sDSField{"last_timestamp", types.StringType, "When the event was last recorded."},
			),
			"warning_count": k8sDSComputedInt("Number of returned events of type `Warning`, which is what usually explains a workload that will not start."),
		},
	}, nil, kubernetesEventsRead)
}

func kubernetesEventsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	path := fmt.Sprintf("/kubernetes/%d/events", envID)
	if namespace != "" {
		path = fmt.Sprintf("/kubernetes/%d/namespaces/%s/events", envID, namespace)
	}
	query := ""
	if v, ok := d.GetOk("resource_id"); ok && namespace == "" {
		q := url.Values{}
		q.Set("resourceId", v.(string))
		query = "?" + q.Encode()
	}

	var raw []struct {
		Type           string `json:"type"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
		Count          int    `json:"count"`
		Namespace      string `json:"namespace"`
		FirstTimestamp string `json:"firstTimestamp"`
		LastTimestamp  string `json:"lastTimestamp"`
		InvolvedObject struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"involvedObject"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+query, nil, &raw); err != nil {
		return fmt.Errorf("failed to read Kubernetes events: %w", err)
	}

	events := make([]map[string]interface{}, len(raw))
	warnings := 0
	for i, e := range raw {
		ns := e.Namespace
		if ns == "" {
			ns = e.InvolvedObject.Namespace
		}
		if e.Type == "Warning" {
			warnings++
		}
		events[i] = map[string]interface{}{
			"type": e.Type, "reason": e.Reason, "message": e.Message, "count": e.Count,
			"namespace": ns, "kind": e.InvolvedObject.Kind, "name": e.InvolvedObject.Name,
			"uid":             e.InvolvedObject.UID,
			"first_timestamp": e.FirstTimestamp, "last_timestamp": e.LastTimestamp,
		}
	}

	if err := d.SetFields(map[string]interface{}{
		"events":        events,
		"warning_count": warnings,
	}); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/events/%s", envID, namespace, strconv.FormatInt(makeTimestamp(), 10)))
	return nil
}
