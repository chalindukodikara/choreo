# Implementation: Make `workflowPlaneRef` Defaulted on Workflow and ClusterWorkflow CRDs

## Approach

Used `+kubebuilder:default` CRD structural schema defaulting instead of webhooks. The API server automatically populates `workflowPlaneRef` when omitted. The field remains a pointer (`*WorkflowPlaneRef`) with `omitempty` so existing YAML without the field works seamlessly — the API server fills in the default on create/update.

## Changes Made

### 1. CRD Type Definitions

- **`api/v1alpha1/workflow_types.go`**: Added `+kubebuilder:default={kind: "ClusterWorkflowPlane", name: "default"}` to `WorkflowPlaneRef`. Kept `*WorkflowPlaneRef` pointer with `+optional` and `omitempty`.
- **`api/v1alpha1/clusterworkflow_types.go`**: Same treatment for `*ClusterWorkflowPlaneRef`.

### 2. Removed Fallback Logic — `internal/controller/reference.go`

- **`ResolveWorkflowPlane()`**: Removed the `if ref == nil` fallback chain that tried namespace WorkflowPlane "default" then ClusterWorkflowPlane "default". Now returns a defensive error if ref is nil, since CRD defaulting ensures it's always populated.
- **`GetWorkflowSpec()`**: Removed the `else` branch that injected `{ClusterWorkflowPlane, "default"}` when ClusterWorkflow's `WorkflowPlaneRef` was nil. Now only maps when ref is non-nil.

### 3. API Service — `internal/openchoreo-api/services/workflowrun/service.go`

- Replaced `getWorkflowPlane()` (blindly listed first WorkflowPlane in namespace) with `resolveWorkflowPlane()` that uses `controller.ResolveWorkflowPlane` to respect the workflow's `workflowPlaneRef`.
- Added `resolveWorkflowPlaneRef()` to look up a workflow's ref from a `WorkflowRunConfig`.
- Updated `getWorkflowPlaneClient()` to accept a `*WorkflowPlaneRef` and use `WorkflowPlaneResult.GetK8sClient()`.
- Updated `GetWorkflowRunLogs()` and `GetWorkflowRunEvents()` to resolve the workflow's `workflowPlaneRef` before calling internal functions.
- Added `controller` import for `ResolveWorkflowPlane` and `ResolveWorkflow`.

### 4. Legacy Component Service — `internal/openchoreo-api/legacyservices/component_service.go`

- Early-return with "not configured" message when component has no workflow, instead of trying nil-ref resolution that would now error.

### 5. Controller Callers

- **`internal/controller/workflowrun/controller_finalize.go`**: When the workflow is already deleted, skip workflow plane cleanup entirely and remove the finalizer. No longer guesses a default workflow plane for cleanup.
- **`internal/occ/cmd/workflowrun/logs.go`**: `resolveWorkflowPlaneObsRef()` — removed the `if workflowName != ""` guard (workflowName is validated non-empty at the call site, line 185-187) and removed the fallback logic that tried namespaced WorkflowPlane "default" then ClusterWorkflowPlane "default". The function now resolves directly from the workflow's `workflowPlaneRef` (guaranteed non-nil by CRD defaulting) and returns `(nil, nil)` only if the resolved workflow plane has no observability plane ref. Updated `resolveObserverURL()` comment to reflect no fallback is needed.

### 6. OpenAPI Spec — `openapi/openchoreo-api.yaml`

- Added description about default behavior for both `WorkflowSpec.workflowPlaneRef` and `ClusterWorkflowSpec.workflowPlaneRef`. Field remains optional (not in `required` list) since the CRD handles defaulting.

### 7. Tests — `internal/controller/reference_test.go`

- Replaced 5 nil-ref fallback tests with 2 nil-ref-returns-error tests (`TestResolveWorkflowPlane_WithNilRef_ReturnsError`, `TestResolveWorkflowPlane_NilRef_ReturnsError`).
- Updated `TestWorkflowResult_GetWorkflowSpec_FromClusterWorkflow_NilWorkflowPlaneRef` to verify nil stays nil (not defaulted in code — CRD handles it).

### 8. Generated Files

- `api/v1alpha1/zz_generated.deepcopy.go` — regenerated via `make generate`.
- CRD YAMLs in `config/crd/bases/` — regenerated via `make manifests`, now include `default: {kind: ClusterWorkflowPlane, name: default}`.

## Remaining

- Run `make openapi-codegen` to regenerate Go models from the updated OpenAPI spec.
- Run `make samples-gen` if sample YAMLs reference workflow resources.
- Run `make code.gen-check` to verify all generated code is up to date.
- Verify Helm chart CRDs are updated (they copy from `config/crd/bases/`).
