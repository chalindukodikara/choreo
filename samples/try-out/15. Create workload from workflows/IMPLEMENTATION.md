# Create Workload from Workflows — Implementation Summary

## Problem

The workflowrun controller had special-case logic to read the Argo Workflow output, look for a `generate-workload-cr` step with a `workload-cr` output parameter, and create the Workload CR directly. This tight coupling between the controller and the workflow template made the system fragile and hard to extend.

## Solution

Move workload creation out of the controller and into the workflow template itself. The template now calls the OpenChoreo API server to create/update the workload, authenticating via OAuth (Thunder IDP). The controller no longer needs to know about workload generation.

## What was implemented

### 1. Thunder IDP — Workflow OAuth Application

**File:** `install/k3d/common/values-thunder.yaml` (bootstrap script `58-workflow-app.sh`)

Registers an OAuth application in Thunder for workflow-to-API-server authentication:
- **client_id:** `openchoreo-workflow-client`
- **client_secret:** `openchoreo-workflow-secret`
- **grant_type:** `client_credentials`
- **token_endpoint_auth_method:** `client_secret_post`

The bootstrap script is idempotent — it checks if the app exists and updates it, or creates it if missing.

### 2. Authorization — Workflow Role and Binding

**File:** `install/helm/openchoreo-control-plane/values.yaml`

Added a new `workflow` role with scoped permissions:
```yaml
- name: workflow
  actions:
    - "workload:create"
    - "workload:update"
    - "workflowrun:view"
    - "workflowrun:update"
```

Added a binding that maps the `openchoreo-workflow-client` subject to this role:
```yaml
- name: workflow-binding
  roleRef:
    name: workflow
  entitlement:
    claim: sub
    value: openchoreo-workflow-client
  effect: allow
```

### 3. Quick-start Values — Workflow Binding

**File:** `install/quick-start/.values-cp.yaml`

Added the `workflow-binding` mapping to the quick-start control plane values. Since Helm replaces lists entirely, the quick-start override must include all mappings explicitly.

### 4. ClusterWorkflowTemplate — `generate-workload`

**File:** `samples/getting-started/workflow-templates/generate-workload.yaml`

New ClusterWorkflowTemplate with a `generate-workload-cr` step that:

1. **Generates the workload CR** using `occ workload create` with the component's `workload.yaml` descriptor (if present) or without it.
2. **Converts** the CR YAML to JSON (stripping `apiVersion`/`kind`) using `yq` and `jq`.
3. **Gets an OAuth token** from Thunder via `client_credentials` grant.
4. **Creates or updates** the workload via `POST /api/v1/namespaces/{namespaceName}/workloads` (falls back to `PUT` on 409 Conflict).
5. **Annotates the WorkflowRun** with the workload CR JSON via `GET` then `PUT` on the workflowrun endpoint.

Configurable parameters (with k3d defaults):
| Parameter | Default (k3d) |
|---|---|
| `oauth-token-url` | `http://host.k3d.internal:8080/oauth2/token` |
| `oauth-host-header` | `thunder.openchoreo.localhost` |
| `oauth-client-id` | `openchoreo-workflow-client` |
| `oauth-client-secret` | `openchoreo-workflow-secret` |
| `api-server-url` | `http://host.k3d.internal:8080` |
| `api-server-host-header` | `api.openchoreo.localhost` |

### 5. WorkflowRun API — Update Endpoint

**Files:**
- `openapi/openchoreo-api.yaml` — `PUT /api/v1/namespaces/{namespaceName}/workflowruns/{runName}` (operationId: `updateWorkflowRun`)
- `internal/openchoreo-api/api/handlers/workflows.go` — `Handler.UpdateWorkflowRun` implementation
- `internal/openchoreo-api/services/workflowrun/service.go` — `UpdateWorkflowRun` service method
- `internal/openchoreo-api/services/workflowrun/service_authz.go` — authz wrapper

The route is auto-registered by the oapi-codegen generated `HandlerFromMuxWithBaseURL` in `server.gen.go`.

### 6. Workload API — CRUD Endpoints

**Files:**
- `openapi/openchoreo-api.yaml` — Workload CRUD endpoints under `/api/v1/namespaces/{namespaceName}/workloads`
- `internal/openchoreo-api/api/handlers/workloads.go` — Handler implementations
- `internal/openchoreo-api/services/workload/` — Service layer with authz

### 7. Controller Cleanup

**File:** `internal/controller/workflowrun/controller.go`

Removed from the controller:
- `hasGenerateWorkloadTask()` — checked if workflow had the special step
- `extractWorkloadCRFromRunResource()` — extracted workload YAML from Argo node outputs
- `generateWorkloadTaskName` / `workloadCRParamName` constants
- `ConditionWorkloadUpdated` / `ReasonWorkloadUpdated` / `ReasonWorkloadUpdateFailed` conditions
- `setWorkloadUpdatedCondition()` / `setWorkloadUpdateFailedCondition()` / `isWorkloadUpdated()` helpers

The reconcile loop now simply:
1. Sets `CompletedAt` when the workflow completes and returns
2. No longer inspects workflow outputs or creates workloads

**File:** `internal/controller/workflowrun/controller_unit_test.go`

Removed tests referencing deleted symbols:
- `TestHasGenerateWorkloadTask`
- `TestExtractWorkloadCRFromRunResource`
- `TestIsWorkloadUpdated`
- Condition sub-tests for `setWorkloadUpdatedCondition` / `setWorkloadUpdateFailedCondition`

**File:** `internal/controller/workflowrun/controller_integration_test.go`

Removed the `"WorkloadUpdated condition causes early return"` integration test context.

## Architecture (Before vs After)

### Before
```
Argo Workflow (generate-workload-cr step)
  -> outputs workload-cr YAML as parameter
  -> WorkflowRun controller reads Argo output
  -> Controller creates Workload CR directly
```

### After
```
Argo Workflow (generate-workload-cr step)
  -> Gets OAuth token from Thunder
  -> Calls API server POST /workloads (or PUT on conflict)
  -> Annotates WorkflowRun with workload data
  -> Controller only tracks workflow completion
```

## Files Changed (Key)

| Area | Files |
|---|---|
| Thunder IDP | `install/k3d/common/values-thunder.yaml` |
| Authz roles | `install/helm/openchoreo-control-plane/values.yaml` |
| Quick-start | `install/quick-start/.values-cp.yaml` |
| Workflow template | `samples/getting-started/workflow-templates/generate-workload.yaml` |
| Publish image (k3d) | `samples/getting-started/workflow-templates/publish-image-k3d.yaml` |
| Publish image (prod) | `samples/getting-started/workflow-templates/publish-image.yaml` |
| OpenAPI spec | `openapi/openchoreo-api.yaml` |
| API handlers | `internal/openchoreo-api/api/handlers/workflows.go`, `workloads.go` |
| API services | `internal/openchoreo-api/services/workflowrun/`, `workload/` |
| Controller | `internal/controller/workflowrun/controller.go`, `controller_conditions.go` |
| Tests | `internal/controller/workflowrun/controller_unit_test.go`, `controller_integration_test.go` |
| E2E values | `test/e2e/k3d/values-thunder.yaml`, `test/e2e/k3d/values-cp.yaml` |