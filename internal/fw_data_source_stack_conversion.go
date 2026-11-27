package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var newStackConversionDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_stack_conversion",
	schema:     dsStackConversionSchema,
	read:       dsStackConversionRead,
})

func dsStackConversionSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"files":         schema.MapAttribute{ElementType: types.StringType, Computed: true, Description: "The converted files, keyed by file name."},
			"namespace":     schema.StringAttribute{Optional: true, Description: "Namespace to generate the Kubernetes resources into."},
			"stack_id":      schema.Int64Attribute{Required: true, Description: "Identifier of the Compose stack to convert."},
			"target_format": schema.StringAttribute{Required: true, Description: "What to convert to: `kubernetes` for manifests or `helm` for a Helm chart.", Validators: []validator.String{dkStringInSlice("kubernetes", "helm")}},
		},
	}
}

func dsStackConversionRead(ctx context.Context, d *dkBag, client *APIClient) error {
	stackID := dkInt(d, "stack_id")
	targetFormat := dkString(d, "target_format")

	payload := map[string]interface{}{"targetFormat": targetFormat}
	if v, ok := d.GetOk("namespace"); ok && v.(string) != "" {
		payload["namespace"] = v.(string)
	}

	var response struct {
		Files map[string]string `json:"files"`
	}
	convertURL := fmt.Sprintf("%s/stacks/%d/convert", client.Endpoint, stackID)
	if err := doJSON(ctx, client, http.MethodPost, convertURL, payload, &response); err != nil {
		return fmt.Errorf("failed to convert stack %d to %s: %w", stackID, targetFormat, err)
	}

	files := make(map[string]interface{}, len(response.Files))
	for name, content := range response.Files {
		files[name] = content
	}

	if err := d.SetFields(map[string]interface{}{"files": files}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-stack-conversion-%d-%s", stackID, targetFormat))
	return nil
}
