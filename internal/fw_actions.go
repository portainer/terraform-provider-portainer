package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// The actions mirroring the one-shot resources (see fw_actions_helpers.go).
// Each invoke runs the function the resource's Create runs; the bag-backed
// resources (invoke unset) share their Create function as it is.

// actionDiagsOnly adapts an operation that reports through diagnostics.
func actionDiagsOnly[M any](run func(ctx context.Context, client *APIClient, m *M, diags *diag.Diagnostics)) actionInvokeFunc {
	return withModel(func(ctx context.Context, client *APIClient, m *M, _ *actionInput, resp *action.InvokeResponse) {
		run(ctx, client, m, &resp.Diagnostics)
	})
}

var (
	newEndpointSnapshotAction = newResourceAction(&resourceActionDef{
		resource:    newEndpointSnapshotResource,
		description: "Takes a snapshot of one environment, or of every environment when `endpoint_id` is unset.",
		invoke:      actionDiagsOnly(endpointSnapshotRun),
	})

	newBackupAction = newResourceAction(&resourceActionDef{
		resource:    newBackupResource,
		description: "Downloads an encrypted backup of the Portainer instance to a local file.",
		required:    []string{"password"},
		invoke: withModel(func(ctx context.Context, client *APIClient, m *backupResourceModel, in *actionInput, resp *action.InvokeResponse) {
			backupRun(ctx, client, m, in.Config, &resp.Diagnostics)
		}),
	})

	newBackupS3Action = newResourceAction(&resourceActionDef{
		resource:    newBackupS3Resource,
		description: "Backs the Portainer instance up to an S3 or S3-compatible bucket once. Business Edition only.",
		required:    []string{"secret_access_key", "password"},
		invoke: withModel(func(ctx context.Context, client *APIClient, m *backupS3ResourceModel, in *actionInput, resp *action.InvokeResponse) {
			backupS3Run(ctx, client, m, in.Config, &resp.Diagnostics)
		}),
	})

	newBackupLocalRunAction = newResourceAction(&resourceActionDef{
		resource:    newBackupLocalRunResource,
		description: "Runs a local backup once, outside the schedule. Business Edition only.",
		invoke: withModel(func(ctx context.Context, client *APIClient, m *backupLocalRunModel, _ *actionInput, resp *action.InvokeResponse) {
			backupLocalRun(ctx, client, m, &resp.Diagnostics)
			// The status read after the run is informational for the resource;
			// the action has nowhere else to report it.
			if !resp.Diagnostics.HasError() && !m.Path.IsNull() {
				sendProgress(resp, fmt.Sprintf("Local backup written to %s (failed: %t)", m.Path.ValueString(), m.Failed.ValueBool()))
			}
		}),
	})

	newBackupAzureExecuteAction = newResourceAction(&resourceActionDef{
		resource:    newBackupAzureExecuteResource,
		description: "Runs an Azure Blob backup once, outside the schedule. Business Edition only.",
		invoke: withModel(func(ctx context.Context, client *APIClient, m *backupAzureExecuteModel, in *actionInput, resp *action.InvokeResponse) {
			backupAzureExecuteRun(ctx, client, m, in.Config, &resp.Diagnostics)
		}),
	})

	newWebhookExecuteAction = newResourceAction(&resourceActionDef{
		resource:    newWebhookExecuteResource,
		description: "Calls a Portainer webhook: a service webhook by token, or the Git update webhook of a stack or an edge stack.",
		invoke:      actionDiagsOnly(webhookExecuteRun),
	})

	newEdgeStackWebhookAction = newResourceAction(&resourceActionDef{
		resource:    newEdgeStackWebhookResource,
		description: "Triggers the webhook of an edge stack, which makes Portainer pull its Git repository and redeploy.",
		invoke:      actionDiagsOnly(edgeStackWebhookRun),
	})

	newContainerExecAction = newResourceAction(&resourceActionDef{
		resource:    newContainerExecResource,
		description: "Runs a command in a container (or in the running task of a Swarm service). The command output is reported as progress.",
	})

	newDeployAction = newResourceAction(&resourceActionDef{
		resource:    newDeployResource,
		description: "Rolls an image revision out to services of a stack and optionally into a stack environment variable. The summary is reported as progress.",
	})

	newCheckAction = newResourceAction(&resourceActionDef{
		resource:    newCheckResource,
		description: "Waits until the services of a stack run the expected revision in the desired state, and fails when they do not. Each check is reported as progress.",
		invoke: withModel(func(ctx context.Context, client *APIClient, m *checkResourceModel, _ *actionInput, resp *action.InvokeResponse) {
			if err := checkRun(ctx, client, m, progressWriter{resp: resp, skip: "DEBUG:"}); err != nil {
				resp.Diagnostics.AddError("Check failed", err.Error())
			}
		}),
	})

	newEndpointServiceUpdateAction = newResourceAction(&resourceActionDef{
		resource:    newEndpointServiceUpdateResource,
		description: "Force-updates a Docker Swarm service, optionally pulling its image first.",
		invoke:      actionDiagsOnly(endpointServiceUpdateRun),
	})

	newEndpointAssociationAction = newResourceAction(&resourceActionDef{
		resource:    newEndpointAssociationResource,
		description: "De-associates an Edge environment from its agent, keeping the environment record.",
		invoke:      actionDiagsOnly(endpointAssociationRun),
	})

	newKubernetesNodeDrainAction = newResourceAction(&resourceActionDef{
		resource:    newKubernetesNodeDrainResource,
		description: "Drains a Kubernetes node, evicting its pods. The node stays cordoned afterwards.",
		invoke: withModel(func(ctx context.Context, client *APIClient, m *kubernetesNodeDrainModel, _ *actionInput, resp *action.InvokeResponse) {
			sendProgress(resp, fmt.Sprintf("Draining node %s (up to %d seconds)", m.NodeName.ValueString(), m.TimeoutSeconds.ValueInt64()))
			kubernetesNodeDrainRun(ctx, client, m, &resp.Diagnostics)
		}),
	})

	newKubernetesDeploymentScaleAction = newResourceAction(&resourceActionDef{
		resource:    newKubernetesDeploymentScaleResource,
		description: "Scales an existing Kubernetes deployment to a replica count.",
		descriptions: map[string]string{
			"deployment_name": "Name of the deployment to scale. The deployment must already exist.",
			"replicas":        "Number of replicas to scale the deployment to.",
		},
		invoke: withModel(func(ctx context.Context, client *APIClient, m *kubernetesDeploymentScaleModel, _ *actionInput, resp *action.InvokeResponse) {
			if err := kubernetesDeploymentScaleRun(ctx, client, m); err != nil {
				resp.Diagnostics.AddError("Failed to scale the deployment", err.Error())
			}
		}),
	})

	newKubernetesDeploymentRollbackAction = newResourceAction(&resourceActionDef{
		resource:    newKubernetesDeploymentRollbackResource,
		description: "Rolls a Kubernetes deployment back to an earlier rollout revision, like `kubectl rollout undo`.",
		invoke:      actionDiagsOnly(kubernetesDeploymentRollbackRun),
	})

	newKubernetesClusterUpgradeAction = newResourceAction(&resourceActionDef{
		resource:    newKubernetesClusterUpgradeResource,
		description: "Upgrades a provisioned Kubernetes cluster to the next stable version. Business Edition only.",
		invoke:      actionDiagsOnly(kubernetesClusterUpgradeRun),
	})

	newKubernetesDeleteObjectAction = newResourceAction(&resourceActionDef{
		resource:    newKubernetesDeleteObjectResource,
		description: "Deletes Kubernetes objects of one type by name.",
		invoke:      actionDiagsOnly(kubernetesDeleteObjectRun),
	})

	newStackMigrateAction = newResourceAction(&resourceActionDef{
		resource:    newStackMigrateResource,
		description: "Migrates a stack to another environment.",
	})

	newStackAssociateAction = newResourceAction(&resourceActionDef{
		resource:    newStackAssociateResource,
		description: "Associates an orphaned stack with an environment.",
	})

	newStackDeleteByNameAction = newResourceAction(&resourceActionDef{
		resource:    newStackDeleteByNameResource,
		description: "Removes every Kubernetes stack of a name from an environment. A stack that is already gone is not an error.",
	})

	newAddonRepairAction = newResourceAction(&resourceActionDef{
		resource:    newAddonRepairResource,
		description: "Reissues the credential an installed addon authenticates to Portainer with. Business Edition only.",
		invoke:      actionDiagsOnly(addonRepairRun),
	})

	newAddonCatalogRefreshAction = newResourceAction(&resourceActionDef{
		resource:    newAddonCatalogRefreshResource,
		description: "Drops Portainer's cached addon catalog, so the next read loads it from the configured source. Business Edition 2.45.2 or later.",
		invoke:      actionDiagsOnly(addonCatalogRefreshRun),
	})

	newGitopsWorkflowReconcileAction = newResourceAction(&resourceActionDef{
		resource:    newGitopsWorkflowReconcileResource,
		description: "Asks a GitOps workflow on the reconcile engine to reconcile now. Business Edition 3.0 or later.",
		invoke:      actionDiagsOnly(gitopsWorkflowReconcileRun),
	})

	newOmniNodeRebootAction = newResourceAction(&resourceActionDef{
		resource:    newOmniNodeRebootResource,
		description: "Reboots a node of an Omni cluster. Business Edition only.",
		invoke:      actionDiagsOnly(omniNodeRebootRun),
	})

	newHelmRollbackAction = newResourceAction(&resourceActionDef{
		resource:    newHelmRollbackResource,
		description: "Rolls a Helm release back to an earlier revision.",
		invoke: withModel(func(ctx context.Context, client *APIClient, m *helmRollbackResourceModel, _ *actionInput, resp *action.InvokeResponse) {
			if m.Wait.ValueBool() || m.WaitForJobs.ValueBool() {
				sendProgress(resp, fmt.Sprintf("Rolling back Helm release %s and waiting for it to be ready", m.ReleaseName.ValueString()))
			}
			helmRollbackRun(ctx, client, m, &resp.Diagnostics)
		}),
	})

	newUserMembershipsSyncAction = newResourceAction(&resourceActionDef{
		resource:    newUserMembershipsSyncResource,
		description: "Refreshes a user's team memberships from the configured LDAP/AD or OAuth provider. Business Edition only.",
		invoke:      actionDiagsOnly(userMembershipsSyncRun),
	})
)
